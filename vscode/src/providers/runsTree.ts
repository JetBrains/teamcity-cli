import * as vscode from "vscode";
import { exec, getServerUrl } from "../cli/runner";
import { log } from "../extension";
import { buildItem } from "./buildItem";
import { argsFor } from "../util";
import type { BuildList, Build } from "../types";

const FILTERS = [
  { label: "All", value: "" },
  { label: "Failed", value: "failure" },
  { label: "Running", value: "running" },
  { label: "Successful", value: "success" },
  { label: "Queued", value: "queued" },
  { label: "Canceled", value: "canceled" },
  { label: "Error", value: "error" },
];

export class RunsTreeProvider implements vscode.TreeDataProvider<Build> {
  private _onDidChange = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChange.event;
  private statusFilter = "";
  private view: vscode.TreeView<Build> | undefined;

  refresh() { this._onDidChange.fire(); }
  setView(view: vscode.TreeView<Build>) { this.view = view; }

  async pickFilter() {
    const picked = await vscode.window.showQuickPick(FILTERS, { placeHolder: "Filter runs by status" });
    if (!picked) return;
    this.statusFilter = picked.value;
    if (this.view) this.view.description = picked.value ? `(${picked.label})` : "";
    this.refresh();
  }

  getTreeItem(build: Build): vscode.TreeItem { return buildItem(build); }

  async getChildren(): Promise<Build[]> {
    if (!getServerUrl()) return [];
    try {
      const list = await exec<BuildList>(argsFor.runList(this.statusFilter));
      log(`Runs loaded: ${list.build?.length ?? 0}${this.statusFilter ? ` (${this.statusFilter})` : ""}`);
      return list.build ?? [];
    } catch (e) {
      log(`Runs error: ${e instanceof Error ? e.message : e}`);
      return [];
    }
  }
}
