import * as vscode from "vscode";
import { exec, getServerUrl, webUrl, streamToOutput } from "../cli/runner";
import { log } from "../extension";
import { getConfig } from "../config";
import { buildItem, BuildNotifier } from "./buildItem";
import type { BuildList, Build } from "../types";

export class FavoritesTreeProvider implements vscode.TreeDataProvider<Build>, vscode.Disposable {
  private _onDidChange = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChange.event;
  private timer: ReturnType<typeof setInterval> | undefined;
  private outputChannel: vscode.OutputChannel | undefined;
  private notifier = new BuildNotifier(() => this.outputChannel, webUrl, streamToOutput);

  refresh() { this._onDidChange.fire(); }
  setOutputChannel(ch: vscode.OutputChannel) { this.outputChannel = ch; }

  startPolling() {
    this.timer = setInterval(() => this.refresh(), getConfig().pollInterval);
  }

  getTreeItem(build: Build): vscode.TreeItem { return buildItem(build); }

  async getChildren(): Promise<Build[]> {
    if (!getServerUrl()) return [];
    try {
      const list = await exec<BuildList>(["run", "list", "--favorites", "--json"]);
      const builds = list.build ?? [];
      log(`Favorites loaded: ${builds.length}`);
      this.notifier.seed(builds);
      for (const b of builds) this.notifier.observe(b);
      return builds;
    } catch (e) {
      log(`Favorites error: ${e instanceof Error ? e.message : e}`);
      return [];
    }
  }

  dispose() { if (this.timer) clearInterval(this.timer); }
}
