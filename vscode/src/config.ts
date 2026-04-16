import * as vscode from "vscode";

export function getConfig() {
  const cfg = vscode.workspace.getConfiguration("teamcity");
  return {
    defaultServer: cfg.get<string>("defaultServer", "https://cli.teamcity.com"),
    cliPath: cfg.get<string>("cliPath", ""),
    pollInterval: cfg.get<number>("pollInterval", 30) * 1000,
    autoRefresh: cfg.get<boolean>("autoRefresh", true),
    enableCodeLens: cfg.get<boolean>("enableCodeLens", true),
  };
}
