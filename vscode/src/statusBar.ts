import * as vscode from "vscode";
import { exec, webUrl, streamToOutput } from "./cli/runner";
import { log } from "./extension";
import { getConfig } from "./config";
import { BuildNotifier } from "./providers/buildItem";
import { backoffDelay } from "./util";
import type { BuildList, Build } from "./types";

export class StatusBarManager implements vscode.Disposable {
  private item: vscode.StatusBarItem;
  private timer: ReturnType<typeof setInterval> | undefined;
  private failCount = 0;
  private outputChannel: vscode.OutputChannel | undefined;
  private notifier = new BuildNotifier(() => this.outputChannel, webUrl, streamToOutput);

  constructor() {
    this.item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    this.item.command = "teamcity.refresh";
    this.item.text = "$(circle-outline) TC";
    this.item.tooltip = "TeamCity: checking...";
    this.item.show();
  }

  setOutputChannel(ch: vscode.OutputChannel) { this.outputChannel = ch; }

  start() {
    this.poll();
    this.reschedule();
  }

  private reschedule() {
    if (this.timer) clearInterval(this.timer);
    const delay = backoffDelay(getConfig().pollInterval, this.failCount);
    this.timer = setInterval(() => this.poll(), delay);
  }

  private async poll() {
    try {
      const list = await exec<BuildList>(["run", "list", "--branch", "@this", "--limit", "1", "--json"]);
      if (this.failCount > 0) {
        log("Reconnected to TeamCity.");
        vscode.window.showInformationMessage("TeamCity: reconnected");
        vscode.commands.executeCommand("teamcity.refreshInternal");
      }
      this.failCount = 0;
      this.reschedule();

      const build = list.build?.[0];
      if (!build) {
        this.notifier.seed([]);
        this.item.text = "$(circle-outline) TC";
        this.item.tooltip = "TeamCity: no runs for this branch";
        return;
      }
      this.notifier.seed(list.build ?? []);
      this.notifier.observe(build);
      this.updateItem(build);
    } catch {
      this.failCount++;
      this.reschedule();
      this.item.text = "$(circle-outline) TC";
      this.item.tooltip = `TeamCity: reconnecting (attempt ${this.failCount})...`;
    }
  }

  private updateItem(build: Build): void {
    const num = build.number ?? build.id;
    if (build.state === "running") {
      const pct = build.percentageComplete ? ` ${build.percentageComplete}%` : "";
      this.item.text = `$(sync~spin) TC #${num}${pct}`;
      this.item.tooltip = `Running: ${build.buildType?.name ?? build.buildTypeId}`;
    } else if (build.status === "SUCCESS") {
      this.item.text = `$(check) TC #${num}`;
      this.item.tooltip = `Success: ${build.statusText ?? ""}`;
    } else if (build.status === "FAILURE") {
      this.item.text = `$(error) TC #${num}`;
      this.item.tooltip = `Failed: ${build.statusText ?? ""}`;
    } else {
      this.item.text = `$(question) TC #${num}`;
      this.item.tooltip = build.statusText ?? "";
    }
  }

  dispose() {
    if (this.timer) clearInterval(this.timer);
    this.item.dispose();
  }
}
