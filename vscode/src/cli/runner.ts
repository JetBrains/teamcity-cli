import * as vscode from "vscode";
import * as child_process from "child_process";

let cliBinaryPath = "";
let serverUrl = "";
let logFn: ((msg: string) => void) | undefined;

export function setLogFn(fn: (msg: string) => void) { logFn = fn; }
export function setCliPath(p: string) { cliBinaryPath = p; }
export function setServerUrl(url: string) { serverUrl = url; }
export function getServerUrl(): string { return serverUrl; }
export function webUrl(path: string): string { return `${serverUrl}${path}`; }

const BASE_ARGS = ["--no-input", "--no-color"];

function debugLog(msg: string) { logFn?.(msg); }

function cliPath(): string {
  if (!cliBinaryPath) throw new Error("TeamCity CLI not initialized");
  return cliBinaryPath;
}

function cliEnv(includeServer = true): NodeJS.ProcessEnv {
  const env = { ...process.env };
  if (includeServer && serverUrl) env.TEAMCITY_URL = serverUrl;
  return env;
}

export async function exec<T>(args: string[], opts?: { noServer?: boolean }): Promise<T> {
  const allArgs = [...BASE_ARGS, ...args];
  debugLog(`exec: teamcity ${allArgs.join(" ")}`);
  return new Promise((resolve, reject) => {
    child_process.execFile(
      cliPath(), allArgs,
      { timeout: 30_000, maxBuffer: 10 * 1024 * 1024, encoding: "utf-8", env: cliEnv(!opts?.noServer) },
      (err, stdout, stderr) => {
        if (err) {
          const msg = stderr?.trim() || err.message;
          debugLog(`exec error: ${msg}`);
          reject(new Error(msg));
          return;
        }
        try { resolve(JSON.parse(stdout) as T); }
        catch {
          debugLog(`exec parse error: ${stdout.slice(0, 200)}`);
          reject(new Error(`Failed to parse CLI output: ${stdout.slice(0, 200)}`));
        }
      },
    );
  });
}

export function streamToOutput(args: string[], channel: vscode.OutputChannel): void {
  channel.show(true);
  channel.appendLine(`> teamcity ${args.join(" ")}\n`);
  const proc = child_process.spawn(cliPath(), [...BASE_ARGS, ...args], {
    stdio: ["ignore", "pipe", "pipe"],
    env: cliEnv(),
  });
  proc.stdout.on("data", (data: Buffer) => channel.append(data.toString()));
  proc.stderr.on("data", (data: Buffer) => channel.append(data.toString()));
  proc.on("close", (code) => channel.appendLine(`\n[Process exited with code ${code}]`));
}

/**
 * Shell-escape a single argument for POSIX shells (sh/bash/zsh/fish).
 * Plain alphanumerics and common path chars pass through; everything else
 * is wrapped in single quotes with embedded `'` replaced by `'\''`.
 */
export function shellEscape(arg: string): string {
  if (arg === "") return "''";
  if (/^[\w@%+=:,./-]+$/.test(arg)) return arg;
  return `'${arg.replace(/'/g, "'\\''")}'`;
}

/** Build the literal command string sent to the user's shell. */
export function buildTerminalCommand(cli: string, args: string[]): string {
  return [cli, "--no-color", ...args].map(shellEscape).join(" ");
}

export function openTerminal(name: string, args: string[]): vscode.Terminal {
  const terminal = vscode.window.createTerminal({
    name: `TeamCity: ${name}`,
    env: serverUrl ? { TEAMCITY_URL: serverUrl } : undefined,
  });
  terminal.show();
  terminal.sendText(buildTerminalCommand(cliPath(), args), true);
  return terminal;
}

export async function background(args: string[], successMsg: string): Promise<void> {
  try {
    await new Promise<void>((resolve, reject) => {
      child_process.execFile(
        cliPath(), [...BASE_ARGS, ...args],
        { timeout: 15_000, encoding: "utf-8", env: cliEnv() },
        (err, _stdout, stderr) => err ? reject(new Error(stderr?.trim() || err.message)) : resolve(),
      );
    });
    vscode.window.showInformationMessage(`TeamCity: ${successMsg}`);
  } catch (e) {
    vscode.window.showErrorMessage(`TeamCity: ${e instanceof Error ? e.message : String(e)}`);
  }
}
