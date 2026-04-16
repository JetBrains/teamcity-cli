import * as vscode from "vscode";
import { ensureCli } from "./cli/binary";
import { setCliPath, setServerUrl, getServerUrl, setLogFn, exec } from "./cli/runner";
import { getConfig } from "./config";
import { argsFor, pickServer } from "./util";
import { PipelinesTreeProvider } from "./providers/pipelinesTree";
import { RunsTreeProvider } from "./providers/runsTree";
import { QueueTreeProvider } from "./providers/queueTree";
import { AgentsTreeProvider } from "./providers/agentsTree";
import { FavoritesTreeProvider } from "./providers/favoritesTree";
import { registerCodeLens } from "./providers/pipelineCodeLens";
import { StatusBarManager } from "./statusBar";
import { registerCommands } from "./commands";
import type { AuthStatus } from "./types";

let outputChannel: vscode.OutputChannel;

export function log(msg: string) {
  outputChannel?.appendLine(`[${new Date().toLocaleTimeString()}] ${msg}`);
}

export async function activate(context: vscode.ExtensionContext) {
  outputChannel = vscode.window.createOutputChannel("TeamCity");
  context.subscriptions.push(outputChannel);
  setLogFn(log);
  log("Extension activating...");

  const pipelines = new PipelinesTreeProvider();
  const runs = new RunsTreeProvider();
  const queue = new QueueTreeProvider();
  const agents = new AgentsTreeProvider();
  const favorites = new FavoritesTreeProvider();
  const statusBar = new StatusBarManager();

  statusBar.setOutputChannel(outputChannel);
  favorites.setOutputChannel(outputChannel);

  const runsView = vscode.window.createTreeView("teamcity.runs", { treeDataProvider: runs });
  runs.setView(runsView);

  context.subscriptions.push(
    statusBar,
    favorites,
    runsView,
    vscode.window.registerTreeDataProvider("teamcity.pipelines", pipelines),
    vscode.window.registerTreeDataProvider("teamcity.queue", queue),
    vscode.window.registerTreeDataProvider("teamcity.agents", agents),
    vscode.window.registerTreeDataProvider("teamcity.favorites", favorites),
  );
  log("Tree providers registered.");

  const refreshAll = () => {
    log("Refreshing all views...");
    pipelines.refresh(); runs.refresh(); queue.refresh(); agents.refresh(); favorites.refresh();
  };
  context.subscriptions.push(vscode.commands.registerCommand("teamcity.refreshInternal", refreshAll));

  registerCommands(context, runs, outputChannel);
  if (getConfig().enableCodeLens) registerCodeLens(context);
  log("Commands registered.");

  try {
    const bin = await ensureCli(context);
    setCliPath(bin);
    log(`CLI binary: ${bin}`);
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    log(`CLI error: ${msg}`);
    vscode.window.showErrorMessage(`TeamCity: could not find CLI — ${msg}`);
    await vscode.commands.executeCommand("setContext", "teamcity.authenticated", false);
    return;
  }

  const authenticated = await resolveAuth();
  vscode.commands.executeCommand("workbench.view.extension.teamcity");

  if (authenticated) {
    log("Authenticated — starting polling and loading data.");
    statusBar.start();
    favorites.startPolling();
    refreshAll();
    vscode.window.showInformationMessage(`TeamCity: connected to ${getServerUrl()}`);
  } else {
    log("Not authenticated — showing welcome content.");
  }
  log("Activation complete.");
}

async function resolveAuth(): Promise<boolean> {
  try {
    const servers = await exec<AuthStatus>(argsFor.authStatus(), { noServer: true });
    log(`Auth status: ${servers.length} server(s).`);

    const preferred = pickServer(servers, getConfig().defaultServer);
    if (!preferred) {
      await vscode.commands.executeCommand("setContext", "teamcity.authenticated", false);
      return false;
    }

    setServerUrl(preferred.server);
    log(`Using server: ${preferred.server} (${preferred.user?.username ?? "guest"})`);
    await vscode.commands.executeCommand("setContext", "teamcity.authenticated", true);
    return true;
  } catch (e) {
    log(`Auth check failed: ${e instanceof Error ? e.message : e}`);
    await vscode.commands.executeCommand("setContext", "teamcity.authenticated", false);
    return false;
  }
}

export function deactivate() {}
