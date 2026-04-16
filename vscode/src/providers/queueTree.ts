import * as vscode from "vscode";
import { exec, getServerUrl } from "../cli/runner";
import { log } from "../extension";
import type { BuildQueue, QueuedBuild } from "../types";

export class QueueTreeProvider implements vscode.TreeDataProvider<QueuedBuild> {
  private _onDidChange = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChange.event;
  refresh() { this._onDidChange.fire(); }

  getTreeItem(build: QueuedBuild): vscode.TreeItem {
    const label = build.buildType?.name ?? build.buildTypeId;
    const item = new vscode.TreeItem(label, vscode.TreeItemCollapsibleState.None);
    item.description = build.branchName ?? "";
    item.tooltip = build.waitReason ?? "Waiting in queue";
    item.contextValue = "queued.webUrl";
    item.iconPath = new vscode.ThemeIcon("clock");
    (item as any).runId = String(build.id);
    return item;
  }

  async getChildren(): Promise<QueuedBuild[]> {
    if (!getServerUrl()) { return []; }
    try {
      const q = await exec<BuildQueue>(["queue", "list", "--json"]);
      return q.build ?? [];
    } catch (e: unknown) {
      log(`Queue error: ${e instanceof Error ? e.message : e}`);
      return [];
    }
  }
}
