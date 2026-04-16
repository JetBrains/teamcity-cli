import * as vscode from "vscode";
import { exec, getServerUrl } from "../cli/runner";
import { log } from "../extension";
import type { AgentList, Agent } from "../types";

export class AgentsTreeProvider implements vscode.TreeDataProvider<Agent> {
  private _onDidChange = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChange.event;
  refresh() { this._onDidChange.fire(); }

  getTreeItem(agent: Agent): vscode.TreeItem {
    const item = new vscode.TreeItem(agent.name, vscode.TreeItemCollapsibleState.None);
    const parts: string[] = [];
    parts.push(agent.connected ? "connected" : "disconnected");
    if (!agent.enabled) parts.push("disabled");
    if (!agent.authorized) parts.push("unauthorized");
    item.description = parts.join(", ");
    item.tooltip = agent.build ? `Running: #${agent.build.number ?? agent.build.id}` : agent.pool?.name ?? "";
    const enabledState = agent.enabled ? "enabled" : "disabled";
    item.contextValue = `agent.${enabledState}.webUrl`;
    item.iconPath = agentIcon(agent);
    (item as any).agentId = String(agent.id);
    (item as any).agentName = agent.name;
    return item;
  }

  async getChildren(): Promise<Agent[]> {
    if (!getServerUrl()) { return []; }
    try {
      const list = await exec<AgentList>(["agent", "list", "--json"]);
      return list.agent ?? [];
    } catch (e: unknown) {
      log(`Agents error: ${e instanceof Error ? e.message : e}`);
      return [];
    }
  }
}

function agentIcon(agent: Agent): vscode.ThemeIcon {
  if (!agent.connected) return new vscode.ThemeIcon("debug-disconnect", new vscode.ThemeColor("charts.red"));
  if (!agent.enabled) return new vscode.ThemeIcon("circle-slash");
  if (agent.build) return new vscode.ThemeIcon("sync~spin", new vscode.ThemeColor("charts.green"));
  return new vscode.ThemeIcon("vm", new vscode.ThemeColor("charts.green"));
}
