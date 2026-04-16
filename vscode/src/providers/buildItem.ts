import * as vscode from "vscode";
import type { Build } from "../types";

export function buildIcon(build: Build): vscode.ThemeIcon {
  if (build.state === "running") return new vscode.ThemeIcon("sync~spin", new vscode.ThemeColor("charts.green"));
  if (build.status === "SUCCESS") return new vscode.ThemeIcon("check", new vscode.ThemeColor("charts.green"));
  if (build.status === "FAILURE") return new vscode.ThemeIcon("error", new vscode.ThemeColor("charts.red"));
  return new vscode.ThemeIcon("circle-outline");
}

export function buildItem(build: Build): vscode.TreeItem {
  const label = `#${build.number ?? build.id}`;
  const item = new vscode.TreeItem(label, vscode.TreeItemCollapsibleState.None);
  item.description = build.buildType?.name ?? build.buildTypeId;
  item.tooltip = [
    build.statusText,
    build.branchName ? `Branch: ${build.branchName}` : "",
    build.agent ? `Agent: ${build.agent.name}` : "",
  ].filter(Boolean).join("\n");
  const state = build.state === "running" ? "running" : "finished";
  item.contextValue = `run.${state}.webUrl`;
  item.iconPath = buildIcon(build);
  item.command = {
    command: "teamcity.viewLog",
    title: "View Build Log",
    arguments: [{ runId: String(build.id) }],
  };
  (item as any).runId = String(build.id);
  (item as any).jobId = build.buildTypeId;
  if (build.state === "running" && build.percentageComplete) {
    item.description += ` (${build.percentageComplete}%)`;
  }
  return item;
}

/**
 * Tracks which build IDs we've already announced so we never notify twice
 * for the same build. `observe` is called with every build we see; it emits
 * at most one notification per build.id, and only for terminal states.
 */
export class BuildNotifier {
  private announced = new Set<number>();
  private seeded = false;
  private readonly maxAnnounced = 500;

  constructor(
    private readonly outputChannel: () => vscode.OutputChannel | undefined,
    private readonly webUrl: (path: string) => string,
    private readonly streamToOutput: (args: string[], ch: vscode.OutputChannel) => void,
  ) {}

  /**
   * Seed with current builds to avoid notifying about pre-existing ones on
   * first load. Only already-finished builds are marked as announced —
   * running builds stay open so their eventual completion still notifies once.
   */
  seed(builds: Build[]): void {
    if (this.seeded) return;
    for (const b of builds) {
      if (b.state !== "running") this.announced.add(b.id);
    }
    this.seeded = true;
  }

  /** Observe a build; fires a notification if it's newly finished and not seen before. */
  observe(build: Build): void {
    if (!this.seeded) return;
    if (build.state === "running") return;
    if (this.announced.has(build.id)) return;
    this.announced.add(build.id);
    if (this.announced.size > this.maxAnnounced) {
      // Drop oldest half to keep memory bounded.
      const keep = Array.from(this.announced).slice(-this.maxAnnounced / 2);
      this.announced = new Set(keep);
    }
    void this.notify(build);
  }

  private async notify(build: Build): Promise<void> {
    const verdict = build.status === "SUCCESS" ? "succeeded" : "failed";
    const name = build.buildType?.name ?? build.buildTypeId;
    const action = await vscode.window.showInformationMessage(
      `TeamCity: ${name} #${build.number ?? build.id} ${verdict}`,
      "View Log", "Open in Browser",
    );
    const ch = this.outputChannel();
    if (action === "View Log" && ch) {
      this.streamToOutput(["run", "log", String(build.id), "--raw"], ch);
    } else if (action === "Open in Browser") {
      vscode.env.openExternal(vscode.Uri.parse(this.webUrl(`/build/${build.id}`)));
    }
  }
}
