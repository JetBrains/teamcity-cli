/**
 * Minimal shim of the `vscode` module, sufficient for unit-testing pure logic
 * that imports types or a few runtime helpers. Extend as needed — anything
 * not stubbed here will throw if code under test tries to use it.
 */
export const notifications: Array<{ message: string; actions: string[] }> = [];
export let actionResponse: string | undefined;
export const quickPickCalls: Array<{ items: readonly any[]; options: any }> = [];
export const inputBoxCalls: Array<{ options: any }> = [];
export let quickPickResponses: any[] = [];
export let inputBoxResponses: (string | undefined)[] = [];

export function setNextAction(action: string | undefined): void { actionResponse = action; }
export function setQuickPickResponses(responses: any[]): void { quickPickResponses = [...responses]; }
export function setInputBoxResponses(responses: (string | undefined)[]): void { inputBoxResponses = [...responses]; }
export function resetNotifications(): void {
  notifications.length = 0;
  actionResponse = undefined;
  quickPickCalls.length = 0;
  inputBoxCalls.length = 0;
  quickPickResponses = [];
  inputBoxResponses = [];
}

export const window = {
  showInformationMessage(message: string, ...actions: string[]): Promise<string | undefined> {
    notifications.push({ message, actions });
    return Promise.resolve(actionResponse);
  },
  showWarningMessage(message: string, ..._actions: string[]): Promise<string | undefined> {
    notifications.push({ message, actions: _actions });
    return Promise.resolve(undefined);
  },
  showErrorMessage(message: string, ..._actions: string[]): Promise<string | undefined> {
    notifications.push({ message, actions: _actions });
    return Promise.resolve(undefined);
  },
  showQuickPick(items: any, options?: any): Promise<any> {
    quickPickCalls.push({ items, options });
    const resolved = Array.isArray(items) ? items : [];
    const next = quickPickResponses.shift();
    // Response can be: undefined (cancel), an item from list, or an index.
    if (next === undefined) return Promise.resolve(undefined);
    if (typeof next === "number") return Promise.resolve(resolved[next]);
    return Promise.resolve(next);
  },
  showInputBox(options?: any): Promise<string | undefined> {
    inputBoxCalls.push({ options });
    return Promise.resolve(inputBoxResponses.shift());
  },
};

export const env = {
  openExternal: (_uri: any) => Promise.resolve(true),
};

export const Uri = {
  parse: (s: string) => ({ toString: () => s, path: s }),
};

export class ThemeIcon {
  constructor(public id: string, public color?: ThemeColor) {}
}
export class ThemeColor { constructor(public id: string) {} }

export class EventEmitter<T> {
  private listeners: Array<(e: T) => void> = [];
  event = (l: (e: T) => void) => { this.listeners.push(l); return { dispose() {} }; };
  fire(e: T) { for (const l of this.listeners) l(e); }
}

export enum TreeItemCollapsibleState { None = 0, Collapsed = 1, Expanded = 2 }
export class TreeItem {
  description?: string;
  tooltip?: string;
  contextValue?: string;
  iconPath?: any;
  command?: any;
  constructor(public label: string, public collapsibleState: TreeItemCollapsibleState = TreeItemCollapsibleState.None) {}
}

export const StatusBarAlignment = { Left: 1, Right: 2 };

export class Range {
  constructor(
    public startLine: number,
    public startCharacter: number,
    public endLine: number,
    public endCharacter: number,
  ) {}
}

export class CodeLens {
  constructor(public range: Range, public command: { command: string; title: string; arguments?: any[] }) {}
}

export const languages = {
  registerCodeLensProvider: (_sel: any, _provider: any) => ({ dispose() {} }),
};

export const commands = {
  registerCommand(_id: string, _handler: any) { return { dispose() {} }; },
  executeCommand(..._args: any[]) { return Promise.resolve(undefined); },
  getCommands() { return Promise.resolve([]); },
};

export const workspace = {
  getConfiguration(_section?: string) {
    return {
      get: <T>(_key: string, defaultValue?: T) => defaultValue as T,
    };
  },
};
