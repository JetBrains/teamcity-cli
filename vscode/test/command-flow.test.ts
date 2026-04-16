/**
 * End-to-end command wiring test:
 *   Every command declared in package.json → registered → when invoked with
 *   a realistic node payload → triggers the expected CLI call.
 *
 * This is the "does every button work?" test. It replaces the runner module
 * with a spy so we can inspect every CLI invocation.
 */
import "./_setup";
import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";
import * as path from "node:path";
import Module from "node:module";

// Spy captures for the runner module.
const calls: {
  exec: Array<{ args: string[]; opts?: any }>;
  openTerminal: Array<{ name: string; args: string[] }>;
  background: Array<{ args: string[]; success: string }>;
  streamToOutput: Array<{ args: string[] }>;
  webUrl: Array<string>;
} = { exec: [], openTerminal: [], background: [], streamToOutput: [], webUrl: [] };

let execResult: any = [];

function resetSpies() {
  calls.exec.length = 0; calls.openTerminal.length = 0;
  calls.background.length = 0; calls.streamToOutput.length = 0;
  calls.webUrl.length = 0;
}

// Build a fake runner module and inject it into the require cache.
const runnerPath = require.resolve("../src/cli/runner");
const defaultExec = (args: string[], opts?: any) => {
  calls.exec.push({ args, opts });
  return Promise.resolve(execResult);
};
require.cache[runnerPath] = {
  id: runnerPath, filename: runnerPath, loaded: true,
  children: [], paths: [], parent: null,
  exports: {
    exec: defaultExec,
    setServerUrl: () => {},
    getServerUrl: () => "https://srv.example.com",
    webUrl: (p: string) => { const u = `https://srv.example.com${p}`; calls.webUrl.push(u); return u; },
    openTerminal: (name: string, args: string[]) => { calls.openTerminal.push({ name, args }); return { show() {}, dispose() {} }; },
    streamToOutput: (args: string[]) => { calls.streamToOutput.push({ args }); },
    background: (args: string[], success: string) => { calls.background.push({ args, success }); return Promise.resolve(); },
    setCliPath: () => {},
    setLogFn: () => {},
  },
} as any;
const runnerMod = require.cache[runnerPath]!.exports as any;

// Now load commands + extension (extension exports `log` used by commands).
// eslint-disable-next-line @typescript-eslint/no-var-requires
const { registerCommands } = require("../src/commands");
// eslint-disable-next-line @typescript-eslint/no-var-requires
const shim = require("./vscode-shim");

// Minimal command registry that captures handlers.
const handlers = new Map<string, (...args: any[]) => any>();
(shim.commands as any).registerCommand = (id: string, handler: any) => {
  handlers.set(id, handler);
  return { dispose() {} };
};
(shim.commands as any).executeCommand = (id: string, ..._args: any[]) => {
  // teamcity.refreshInternal is registered in extension.ts; just ignore here.
  return Promise.resolve();
};

const fakeContext: any = { subscriptions: [] };
const fakeRuns: any = { pickFilter: () => Promise.resolve(), refresh: () => {}, setView: () => {} };
const fakeChannel: any = { appendLine: () => {}, append: () => {}, show: () => {} };
registerCommands(fakeContext, fakeRuns, fakeChannel);

beforeEach(() => {
  resetSpies();
  shim.resetNotifications();
  runnerMod.exec = defaultExec;
  execResult = [];
});

// ---------- Auth ----------

test("teamcity.login → opens terminal with auth login + server", async () => {
  await handlers.get("teamcity.login")!();
  assert.equal(calls.openTerminal.length, 1);
  assert.equal(calls.openTerminal[0].name, "Login");
  assert.deepEqual(calls.openTerminal[0].args, ["auth", "login", "-s", "https://cli.teamcity.com"]);
});

test("teamcity.loginGuest → opens terminal with --guest", async () => {
  await handlers.get("teamcity.loginGuest")!();
  assert.deepEqual(calls.openTerminal[0].args, ["auth", "login", "--guest", "-s", "https://cli.teamcity.com"]);
});

test("teamcity.logout → opens terminal with auth logout", async () => {
  await handlers.get("teamcity.logout")!();
  assert.deepEqual(calls.openTerminal[0].args, ["auth", "logout"]);
});

test("teamcity.authStatus → execs auth status --json and notifies", async () => {
  execResult = [{ server: "https://srv.example.com", status: "authenticated", user: { username: "alice" } }];
  await handlers.get("teamcity.authStatus")!();
  assert.deepEqual(calls.exec[0].args, ["auth", "status", "--json"]);
  assert.equal(calls.exec[0].opts?.noServer, true);
  assert.match(shim.notifications[0].message, /logged in to https:\/\/srv\.example\.com as alice/);
});

test("teamcity.authStatus (empty) → warning", async () => {
  execResult = [];
  await handlers.get("teamcity.authStatus")!();
  assert.match(shim.notifications[0].message, /not authenticated/);
});

// ---------- Pipelines ----------

test("teamcity.createPipeline → opens terminal with pipeline create", async () => {
  await handlers.get("teamcity.createPipeline")!();
  assert.deepEqual(calls.openTerminal[0].args, ["pipeline", "create"]);
});

test("teamcity.validatePipeline → streams pipeline validate", async () => {
  await handlers.get("teamcity.validatePipeline")!();
  assert.deepEqual(calls.streamToOutput[0].args, ["pipeline", "validate"]);
});

test("teamcity.pushPipeline → opens terminal with pipeline push", async () => {
  await handlers.get("teamcity.pushPipeline")!();
  assert.deepEqual(calls.openTerminal[0].args, ["pipeline", "push"]);
});

// ---------- Run triggers ----------

test("teamcity.triggerRun with jobId → run start <id> --watch", async () => {
  await handlers.get("teamcity.triggerRun")!({ jobId: "CI_Build" });
  assert.deepEqual(calls.openTerminal[0].args, ["run", "start", "CI_Build", "--watch"]);
});

test("teamcity.triggerRun without jobId → run start (interactive)", async () => {
  await handlers.get("teamcity.triggerRun")!(undefined);
  assert.deepEqual(calls.openTerminal[0].args, ["run", "start"]);
});

test("teamcity.remoteRun: with jobId adds id AND --local-changes", async () => {
  await handlers.get("teamcity.remoteRun")!({ jobId: "CI_Build" });
  assert.deepEqual(calls.openTerminal[0].args, ["run", "start", "CI_Build", "--local-changes"]);
});

test("teamcity.remoteRun: without jobId prompts pipeline → job quick picks", async () => {
  let nth = 0;
  runnerMod.exec = () => {
    nth++;
    if (nth === 1) return Promise.resolve({ pipeline: [{ id: "CI_Root", name: "CI" }] });
    return Promise.resolve({ jobs: { job: [{ id: "test_macos", name: "Test macOS" }] } });
  };
  shim.setQuickPickResponses([
    { pipelineId: "CI_Root" },
    { jobId: "test_macos" },
  ]);

  await handlers.get("teamcity.remoteRun")!(undefined);
  assert.deepEqual(calls.openTerminal[0].args, ["run", "start", "test_macos", "--local-changes"]);
  assert.equal(shim.quickPickCalls.length, 2);
  assert.match(shim.quickPickCalls[0].options.placeHolder, /select a pipeline/i);
  assert.match(shim.quickPickCalls[1].options.placeHolder, /select a job/i);
});

test("teamcity.remoteRun: user cancels pipeline pick → no terminal opened", async () => {
  runnerMod.exec = () => Promise.resolve({ pipeline: [{ id: "CI", name: "CI" }] });
  shim.setQuickPickResponses([undefined]);
  await handlers.get("teamcity.remoteRun")!(undefined);
  assert.equal(calls.openTerminal.length, 0);
});

test("teamcity.remoteRun: CLI failure falls back to input box", async () => {
  runnerMod.exec = () => Promise.reject(new Error("boom"));
  shim.setInputBoxResponses(["Manual_Job_ID"]);
  await handlers.get("teamcity.remoteRun")!(undefined);
  assert.equal(shim.inputBoxCalls.length, 1);
  assert.deepEqual(calls.openTerminal[0].args, ["run", "start", "Manual_Job_ID", "--local-changes"]);
});

test("teamcity.remoteRun: no pipelines → input box fallback", async () => {
  runnerMod.exec = () => Promise.resolve({ pipeline: [] });
  shim.setInputBoxResponses([undefined]);
  await handlers.get("teamcity.remoteRun")!(undefined);
  assert.equal(shim.inputBoxCalls.length, 1);
  assert.equal(calls.openTerminal.length, 0);
});

test("teamcity.remoteRun: selected pipeline has no jobs → warning and abort", async () => {
  let nth = 0;
  runnerMod.exec = () => {
    nth++;
    if (nth === 1) return Promise.resolve({ pipeline: [{ id: "Empty", name: "Empty CI" }] });
    return Promise.resolve({ jobs: { job: [] } });
  };
  shim.setQuickPickResponses([{ pipelineId: "Empty" }]);
  await handlers.get("teamcity.remoteRun")!(undefined);
  assert.equal(calls.openTerminal.length, 0);
  assert.match(shim.notifications[0].message, /no jobs/i);
});

// ---------- Logs ----------

test("teamcity.viewLog → streams run log <id> --raw", async () => {
  await handlers.get("teamcity.viewLog")!({ runId: "123" });
  assert.deepEqual(calls.streamToOutput[0].args, ["run", "log", "123", "--raw"]);
});

test("teamcity.viewFailedTests → streams run tests --failed --json", async () => {
  await handlers.get("teamcity.viewFailedTests")!({ runId: "123" });
  assert.deepEqual(calls.streamToOutput[0].args, ["run", "tests", "123", "--failed", "--json"]);
});

test("teamcity.viewLog without id → no-op", async () => {
  await handlers.get("teamcity.viewLog")!(undefined);
  assert.equal(calls.streamToOutput.length, 0);
});

// ---------- Terminal actions (data-driven table) ----------

test("teamcity.watchRun → run watch <id> --logs", async () => {
  await handlers.get("teamcity.watchRun")!({ runId: "42" });
  assert.deepEqual(calls.openTerminal[0].args, ["run", "watch", "42", "--logs"]);
  assert.equal(calls.openTerminal[0].name, "Watch");
});

test("teamcity.restartRun → run restart <id> --watch", async () => {
  await handlers.get("teamcity.restartRun")!({ runId: "42" });
  assert.deepEqual(calls.openTerminal[0].args, ["run", "restart", "42", "--watch"]);
});

test("teamcity.downloadArtifacts → run download <id>", async () => {
  await handlers.get("teamcity.downloadArtifacts")!({ runId: "42" });
  assert.deepEqual(calls.openTerminal[0].args, ["run", "download", "42"]);
});

test("teamcity.agentTerminal → agent term <name> (uses agentName)", async () => {
  await handlers.get("teamcity.agentTerminal")!({ agentName: "mac-mini-1", id: 77 });
  assert.deepEqual(calls.openTerminal[0].args, ["agent", "term", "mac-mini-1"]);
});

// ---------- Background actions ----------

test("teamcity.cancelRun → run cancel -y in background", async () => {
  await handlers.get("teamcity.cancelRun")!({ runId: "42" });
  assert.deepEqual(calls.background[0].args, ["run", "cancel", "42", "-y"]);
  assert.match(calls.background[0].success, /canceled/i);
});

test("teamcity.pinRun / unpinRun → pin/unpin in background", async () => {
  await handlers.get("teamcity.pinRun")!({ runId: "42" });
  await handlers.get("teamcity.unpinRun")!({ runId: "42" });
  assert.deepEqual(calls.background.map(b => b.args), [
    ["run", "pin", "42"],
    ["run", "unpin", "42"],
  ]);
});

test("teamcity.approveQueued / removeFromQueue / moveToTop → queue ops", async () => {
  await handlers.get("teamcity.approveQueued")!({ runId: "7" });
  await handlers.get("teamcity.removeFromQueue")!({ runId: "7" });
  await handlers.get("teamcity.moveToTop")!({ runId: "7" });
  assert.deepEqual(calls.background.map(b => b.args), [
    ["queue", "approve", "7"],
    ["queue", "remove", "7", "-y"],
    ["queue", "top", "7"],
  ]);
});

test("teamcity.enableAgent / disableAgent → agent ops (prefers agentId)", async () => {
  await handlers.get("teamcity.enableAgent")!({ agentId: "A1", id: 999 });
  await handlers.get("teamcity.disableAgent")!({ agentId: "A1", id: 999 });
  assert.deepEqual(calls.background.map(b => b.args), [
    ["agent", "enable", "A1"],
    ["agent", "disable", "A1"],
  ]);
});

test("background action with no id → no-op", async () => {
  await handlers.get("teamcity.cancelRun")!(undefined);
  await handlers.get("teamcity.cancelRun")!({});
  assert.equal(calls.background.length, 0);
});

// ---------- openInBrowser ----------

test("teamcity.openInBrowser: build → /build/{id}", () => {
  handlers.get("teamcity.openInBrowser")!({ id: 100, buildTypeId: "CI_X" });
  assert.equal(calls.webUrl[0], "https://srv.example.com/build/100");
});

test("teamcity.openInBrowser: pipeline tree node → /pipeline/{id}", () => {
  handlers.get("teamcity.openInBrowser")!({ kind: "pipeline", data: { id: "CI_Root" } });
  assert.equal(calls.webUrl[0], "https://srv.example.com/pipeline/CI_Root");
});

test("teamcity.openInBrowser: job tree node → parent /pipeline/{pipelineId}", () => {
  handlers.get("teamcity.openInBrowser")!({
    kind: "job",
    data: { id: "test_macos", pipelineId: "CI_Root" },
  });
  assert.equal(calls.webUrl[0], "https://srv.example.com/pipeline/CI_Root");
});

test("teamcity.openInBrowser: agent → /agent/{id}", () => {
  handlers.get("teamcity.openInBrowser")!({ id: 5, name: "agent1", connected: true });
  assert.equal(calls.webUrl[0], "https://srv.example.com/agent/5");
});

test("teamcity.openInBrowser: unknown node → no URL opened", () => {
  handlers.get("teamcity.openInBrowser")!({});
  assert.equal(calls.webUrl.length, 0);
});

// ---------- Coverage sanity ----------

test("every declared command has a working handler", async () => {
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const pkg = require("../package.json");
  const declared: string[] = pkg.contributes.commands.map((c: any) => c.command);
  const unregistered: string[] = [];
  for (const id of declared) {
    if (!handlers.has(id)) unregistered.push(id);
  }
  assert.deepEqual(unregistered, [], `unregistered commands: ${unregistered.join(", ")}`);
});
