import * as vscode from "vscode";

const SELECTOR: vscode.DocumentSelector = { language: "yaml", pattern: "**/.teamcity.yml" };

export class PipelineCodeLensProvider implements vscode.CodeLensProvider {
  private _onDidChange = new vscode.EventEmitter<void>();
  readonly onDidChangeCodeLenses = this._onDidChange.event;

  provideCodeLenses(document: vscode.TextDocument): vscode.CodeLens[] {
    const lenses: vscode.CodeLens[] = [];
    const topRange = new vscode.Range(0, 0, 0, 0);

    lenses.push(new vscode.CodeLens(topRange, {
      title: "$(check) Validate Pipeline",
      command: "teamcity.validatePipeline",
    }));
    lenses.push(new vscode.CodeLens(topRange, {
      title: "$(cloud-upload) Push Pipeline",
      command: "teamcity.pushPipeline",
    }));

    let inJobs = false;
    for (let i = 0; i < document.lineCount; i++) {
      const line = document.lineAt(i).text;
      if (/^jobs:\s*$/.test(line)) {
        inJobs = true;
        continue;
      }
      if (inJobs && /^\S/.test(line)) {
        inJobs = false;
        continue;
      }
      if (inJobs) {
        const match = line.match(/^  ([a-zA-Z_][\w-]*):\s*$/);
        if (match) {
          const range = new vscode.Range(i, 0, i, 0);
          lenses.push(new vscode.CodeLens(range, {
            title: "$(play) Run Job",
            command: "teamcity.triggerRun",
            arguments: [{ jobId: match[1] }],
          }));
        }
      }
    }

    return lenses;
  }
}

export function registerCodeLens(context: vscode.ExtensionContext) {
  const provider = new PipelineCodeLensProvider();
  context.subscriptions.push(vscode.languages.registerCodeLensProvider(SELECTOR, provider));
}
