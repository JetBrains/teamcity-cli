import * as vscode from "vscode";
import * as fs from "fs";
import * as path from "path";
import * as crypto from "crypto";
import * as https from "https";
import * as child_process from "child_process";
import { CLI_VERSION, CHECKSUMS } from "./checksums";
import { getConfig } from "../config";

const REPO = "JetBrains/teamcity-cli";

/** Pure: builds the release-asset filename for a given (platform, arch, version). */
export function platformKey(
  platform: NodeJS.Platform = process.platform,
  arch: NodeJS.Architecture = process.arch as NodeJS.Architecture,
  version = CLI_VERSION,
): string {
  const os = platform === "win32" ? "windows" : platform === "darwin" ? "darwin" : "linux";
  const goArch = arch === "arm64" ? "arm64" : "x86_64";
  const ext = os === "windows" ? "zip" : "tar.gz";
  return `teamcity_${version}_${os}_${goArch}.${ext}`;
}

/** Pure: parses `teamcity --version` output and checks it matches a known version. */
export function versionMatches(output: string, expected: string): boolean {
  return output.trim() === `teamcity version ${expected}`;
}

function binDir(context: vscode.ExtensionContext): string {
  return path.join(context.globalStorageUri.fsPath, "bin");
}

function binaryPath(context: vscode.ExtensionContext): string {
  const name = process.platform === "win32" ? "teamcity.exe" : "teamcity";
  return path.join(binDir(context), name);
}

function findOnPath(): string | undefined {
  try {
    const result = child_process.execFileSync(
      process.platform === "win32" ? "where" : "which",
      ["teamcity"],
      { timeout: 3000, encoding: "utf-8" }
    );
    const p = result.trim().split("\n")[0];
    if (p && fs.existsSync(p)) { return p; }
  } catch { /* not on PATH */ }
  return undefined;
}

function isCliUsable(bin: string): boolean {
  try {
    child_process.execFileSync(bin, ["--version"], { timeout: 5000, encoding: "utf-8" });
    return true;
  } catch {
    return false;
  }
}

export async function ensureCli(context: vscode.ExtensionContext): Promise<string> {
  const custom = getConfig().cliPath;
  if (custom) {
    if (!fs.existsSync(custom)) {
      throw new Error(`Custom CLI path not found: ${custom}`);
    }
    return custom;
  }

  const onPath = findOnPath();
  if (onPath && isCliUsable(onPath)) { return onPath; }

  const bin = binaryPath(context);
  if (fs.existsSync(bin) && matchesVersion(bin)) { return bin; }

  return download(context);
}

function matchesVersion(bin: string): boolean {
  try {
    const out = child_process.execFileSync(bin, ["--version"], { timeout: 5000, encoding: "utf-8" });
    return versionMatches(out, CLI_VERSION);
  } catch {
    return false;
  }
}

async function download(context: vscode.ExtensionContext): Promise<string> {
  const key = platformKey();
  const expectedHash = CHECKSUMS[key];
  const verify = expectedHash && !expectedHash.startsWith("PLACEHOLDER");

  const url = `https://github.com/${REPO}/releases/download/v${CLI_VERSION}/${key}`;
  const dir = binDir(context);
  fs.mkdirSync(dir, { recursive: true });
  const archivePath = path.join(dir, key);

  await vscode.window.withProgress(
    { location: vscode.ProgressLocation.Notification, title: `Downloading TeamCity CLI v${CLI_VERSION}...` },
    async () => {
      await downloadFile(url, archivePath);
      if (verify) {
        const actual = await sha256(archivePath);
        if (actual !== expectedHash) {
          fs.unlinkSync(archivePath);
          throw new Error(`Checksum mismatch for ${key}: expected ${expectedHash}, got ${actual}`);
        }
      }
      await extract(archivePath, dir);
      fs.unlinkSync(archivePath);
      const bin = binaryPath(context);
      if (process.platform !== "win32") { fs.chmodSync(bin, 0o755); }
    }
  );

  return binaryPath(context);
}

function downloadFile(url: string, dest: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    const get = (u: string) => {
      https.get(u, { headers: { "User-Agent": "teamcity-vscode" } }, (res) => {
        if (res.statusCode === 302 || res.statusCode === 301) { get(res.headers.location!); return; }
        if (res.statusCode !== 200) { reject(new Error(`Download failed: HTTP ${res.statusCode}`)); return; }
        res.pipe(file);
        file.on("finish", () => { file.close(); resolve(); });
      }).on("error", reject);
    };
    get(url);
  });
}

function sha256(filePath: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash("sha256");
    const stream = fs.createReadStream(filePath);
    stream.on("data", (data) => hash.update(data));
    stream.on("end", () => resolve(hash.digest("hex")));
    stream.on("error", reject);
  });
}

async function extract(archive: string, dest: string): Promise<void> {
  if (archive.endsWith(".zip")) {
    child_process.execFileSync("unzip", ["-o", archive, "-d", dest]);
  } else {
    child_process.execFileSync("tar", ["xzf", archive, "-C", dest]);
  }
}
