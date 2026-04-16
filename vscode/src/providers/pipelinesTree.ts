import * as vscode from "vscode";
import { exec, getServerUrl } from "../cli/runner";
import { log } from "../extension";
import type { PipelineList, Pipeline } from "../types";

type Node =
  | { kind: "pipeline"; data: Pipeline }
  | { kind: "job"; data: { id: string; name: string; pipelineId: string } };

export class PipelinesTreeProvider implements vscode.TreeDataProvider<Node> {
  private _onDidChange = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChange.event;
  refresh() { this._onDidChange.fire(); }

  getTreeItem(node: Node): vscode.TreeItem {
    if (node.kind === "pipeline") {
      const item = new vscode.TreeItem(node.data.name, vscode.TreeItemCollapsibleState.Collapsed);
      item.description = node.data.id;
      item.tooltip = node.data.parentProject?.name;
      item.contextValue = "pipeline.webUrl";
      item.iconPath = new vscode.ThemeIcon("server-process");
      return item;
    }
    const item = new vscode.TreeItem(node.data.name, vscode.TreeItemCollapsibleState.None);
    item.description = node.data.id;
    item.contextValue = "job.webUrl";
    item.iconPath = new vscode.ThemeIcon("gear");
    (item as any).jobId = node.data.id;
    (item as any).pipelineId = node.data.pipelineId;
    return item;
  }

  async getChildren(node?: Node): Promise<Node[]> {
    if (!getServerUrl()) { return []; }
    try {
      if (!node) {
        const list = await exec<PipelineList>(["pipeline", "list", "--json"]);
        log(`Pipelines loaded: ${list.pipeline?.length ?? 0}`);
        return (list.pipeline ?? []).map((p) => ({ kind: "pipeline" as const, data: p }));
      }
      if (node.kind === "pipeline") {
        const detail = await exec<Pipeline>(["pipeline", "view", node.data.id, "--json"]);
        return (detail.jobs?.job ?? []).map((j) => ({
          kind: "job" as const,
          data: { id: j.id, name: j.name, pipelineId: node.data.id },
        }));
      }
      return [];
    } catch (e: unknown) {
      log(`Pipelines error: ${e instanceof Error ? e.message : e}`);
      return [];
    }
  }
}
