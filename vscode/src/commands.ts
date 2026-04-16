import * as vscode from "vscode";
import { exec, setServerUrl, getServerUrl, webUrl, openTerminal, streamToOutput, background } from "./cli/runner";
import { getConfig } from "./config";
import { log } from "./extension";
import { nodeId, browserPathFor, argsFor } from "./util";
import type { RunsTreeProvider } from "./providers/runsTree";
import type { AuthStatus, PipelineList, Pipeline } from "./types";

type Node = Record<string, any> | undefined;

export type BackgroundAction = {
  id: string;
  args: (id: string) => string[];
  success: string;
  refresh?: boolean;
  idKeys?: string[];
};

export type TerminalAction = {
  id: string;
  name: string;
  args: (id?: string) => string[];
  idKeys?: string[];
};

// Background CLI actions that reload views on success.
export const BACKGROUND_ACTIONS: BackgroundAction[] = [
  { id: "teamcity.cancelRun", args: (id) => ["run", "cancel", id, "-y"], success: "Run canceled", refresh: true },
  { id: "teamcity.pinRun", args: (id) => ["run", "pin", id], success: "Run pinned" },
  { id: "teamcity.unpinRun", args: (id) => ["run", "unpin", id], success: "Run unpinned" },
  { id: "teamcity.approveQueued", args: (id) => ["queue", "approve", id], success: "Build approved", refresh: true },
  { id: "teamcity.removeFromQueue", args: (id) => ["queue", "remove", id, "-y"], success: "Removed from queue", refresh: true },
  { id: "teamcity.moveToTop", args: (id) => ["queue", "top", id], success: "Moved to top", refresh: true },
  { id: "teamcity.enableAgent", args: (id) => ["agent", "enable", id], success: "Agent enabled", refresh: true, idKeys: ["agentId", "id"] },
  { id: "teamcity.disableAgent", args: (id) => ["agent", "disable", id], success: "Agent disabled", refresh: true, idKeys: ["agentId", "id"] },
];

// Terminal-based actions that take an optional ID from the current node.
export const TERMINAL_ACTIONS: TerminalAction[] = [
  { id: "teamcity.watchRun", name: "Watch", args: (id) => ["run", "watch", id!, "--logs"] },
  { id: "teamcity.restartRun", name: "Restart", args: (id) => ["run", "restart", id!, "--watch"] },
  { id: "teamcity.downloadArtifacts", name: "Download Artifacts", args: (id) => ["run", "download", id!] },
  {
    id: "teamcity.agentTerminal", name: "Agent Terminal",
    args: (id) => ["agent", "term", id!],
    idKeys: ["agentName", "agentId", "name"],
  },
];

export function registerCommands(
  context: vscode.ExtensionContext,
  runs: RunsTreeProvider,
  outputChannel: vscode.OutputChannel,
): void {
  const reg = (id: string, handler: (...args: any[]) => any) =>
    context.subscriptions.push(vscode.commands.registerCommand(id, handler));

  const trace = (cmd: string, node?: Node) => log(`cmd: ${cmd}${node ? ` ${JSON.stringify(node)}` : ""}`);
  const refresh = () => vscode.commands.executeCommand("teamcity.refreshInternal");

  // Auth & config.
  reg("teamcity.login", () => { trace("login"); openTerminal("Login", argsFor.login(getConfig().defaultServer)); });
  reg("teamcity.loginGuest", () => { trace("loginGuest"); openTerminal("Login (Guest)", argsFor.loginGuest(getConfig().defaultServer)); });
  reg("teamcity.logout", () => { trace("logout"); openTerminal("Logout", argsFor.logout()); });
  reg("teamcity.authStatus", () => showAuthStatus());
  reg("teamcity.selectServer", async () => {
    const url = await vscode.window.showInputBox({ prompt: "TeamCity server URL", value: getConfig().defaultServer });
    if (!url) return;
    trace(`selectServer → ${url}`);
    setServerUrl(url);
    refresh();
  });
  reg("teamcity.refresh", () => { trace("refresh"); refresh(); });
  reg("teamcity.filterRuns", () => runs.pickFilter());

  // Pipelines.
  reg("teamcity.createPipeline", () => { trace("createPipeline"); openTerminal("Create Pipeline", argsFor.pipelineCreate()); });
  reg("teamcity.validatePipeline", () => { trace("validatePipeline"); streamToOutput(argsFor.pipelineValidate(), outputChannel); });
  reg("teamcity.pushPipeline", () => { trace("pushPipeline"); openTerminal("Push Pipeline", argsFor.pipelinePush()); });

  // Runs — trigger variants.
  reg("teamcity.triggerRun", (node: Node) => {
    trace("triggerRun", node);
    openTerminal("Trigger Run", argsFor.triggerRun(nodeId(node, "jobId", "id")));
  });
  reg("teamcity.remoteRun", async (node: Node) => {
    trace("remoteRun", node);
    // --local-changes requires a specific job ID; prompt if not in context.
    const jobId = nodeId(node, "jobId", "id") ?? await pickJob("Remote run");
    if (!jobId) return;
    openTerminal("Remote Run", argsFor.remoteRun(jobId));
  });

  // Logs & tests (stream to output channel).
  reg("teamcity.viewLog", (node: Node) => {
    trace("viewLog", node);
    const id = nodeId(node, "runId", "id");
    if (id) streamToOutput(argsFor.viewLog(id), outputChannel);
  });
  reg("teamcity.viewFailedTests", (node: Node) => {
    trace("viewFailedTests", node);
    const id = nodeId(node, "runId", "id");
    if (id) streamToOutput(argsFor.viewFailedTests(id), outputChannel);
  });

  // Browser.
  reg("teamcity.openInBrowser", (node: Node) => {
    trace("openInBrowser", node);
    const path = browserPathFor(node);
    if (path) { const url = webUrl(path); log(`opening: ${url}`); vscode.env.openExternal(vscode.Uri.parse(url)); }
    else log("openInBrowser: could not determine URL");
  });

  // Data-driven terminal actions.
  for (const a of TERMINAL_ACTIONS) {
    reg(a.id, (node: Node) => {
      trace(a.id, node);
      const id = nodeId(node, ...(a.idKeys ?? ["runId", "id"]));
      if (id) openTerminal(a.name, a.args(id));
    });
  }

  // Data-driven background actions.
  for (const a of BACKGROUND_ACTIONS) {
    reg(a.id, async (node: Node) => {
      trace(a.id, node);
      const id = nodeId(node, ...(a.idKeys ?? ["runId", "id"]));
      if (!id) return;
      await background(a.args(id), a.success);
      if (a.refresh) refresh();
    });
  }
}

/**
 * Two-step quick-pick: pipeline → job. Falls back to a free-form input box
 * if no pipelines are available or the CLI call fails.
 */
async function pickJob(context: string): Promise<string | undefined> {
  try {
    const list = await exec<PipelineList>(argsFor.pipelineList());
    const pipelines = list.pipeline ?? [];
    if (pipelines.length === 0) return askJobIdDirectly(context);

    const pickedPipeline = await vscode.window.showQuickPick(
      pipelines.map((p) => ({ label: p.name, description: p.id, pipelineId: p.id })),
      { placeHolder: `${context} — select a pipeline` },
    );
    if (!pickedPipeline) return undefined;

    const detail = await exec<Pipeline>(argsFor.pipelineView(pickedPipeline.pipelineId));
    const jobs = detail.jobs?.job ?? [];
    if (jobs.length === 0) {
      vscode.window.showWarningMessage(`TeamCity: ${pickedPipeline.label} has no jobs`);
      return undefined;
    }

    const pickedJob = await vscode.window.showQuickPick(
      jobs.map((j) => ({ label: j.name, description: j.id, jobId: j.id })),
      { placeHolder: `${context} — select a job` },
    );
    return pickedJob?.jobId;
  } catch (e) {
    log(`pickJob error: ${e instanceof Error ? e.message : e}`);
    return askJobIdDirectly(context);
  }
}

function askJobIdDirectly(context: string): Thenable<string | undefined> {
  return vscode.window.showInputBox({
    prompt: `${context} — job ID`,
    placeHolder: "e.g., CI_Build_MacOS",
  });
}

async function showAuthStatus(): Promise<void> {
  try {
    const servers = await exec<AuthStatus>(argsFor.authStatus(), { noServer: true });
    const authed = servers.filter((s) => s.status === "authenticated");
    if (authed.length === 0) { vscode.window.showWarningMessage("TeamCity: not authenticated"); return; }
    const srv = getServerUrl() || authed[0].server;
    const user = authed.find((s) => s.server === srv)?.user?.username ?? "guest";
    vscode.window.showInformationMessage(`TeamCity: logged in to ${srv} as ${user}`);
  } catch (e) {
    log(`authStatus error: ${e instanceof Error ? e.message : e}`);
    vscode.window.showWarningMessage("TeamCity: unable to check auth status");
  }
}
