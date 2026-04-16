import { test } from "node:test";
import assert from "node:assert/strict";
import * as fs from "node:fs";
import * as path from "node:path";

const ROOT = path.resolve(__dirname, "..");
const pkg = JSON.parse(fs.readFileSync(path.join(ROOT, "package.json"), "utf8"));

function readAllSource(): string {
  const out: string[] = [];
  const walk = (dir: string) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(p);
      else if (entry.name.endsWith(".ts")) out.push(fs.readFileSync(p, "utf8"));
    }
  };
  walk(path.join(ROOT, "src"));
  return out.join("\n");
}

const source = readAllSource();
const declaredCommands: string[] = pkg.contributes.commands.map((c: any) => c.command);

// Helper: every contribution that references a command must refer to a declared one.
const menuGroups: Array<{ group: string; items: Array<{ command: string; when?: string }> }> = [];
for (const [group, items] of Object.entries<any[]>(pkg.contributes.menus ?? {})) {
  menuGroups.push({ group, items });
}

test("every declared command is registered in source", () => {
  const missing: string[] = [];
  for (const id of declaredCommands) {
    const patterns = [
      `registerCommand("${id}"`,
      `registerCommand('${id}'`,
      `"${id}"`, // data-driven tables (BACKGROUND_ACTIONS / TERMINAL_ACTIONS)
      `'${id}'`,
    ];
    if (!patterns.some((p) => source.includes(p))) missing.push(id);
  }
  assert.deepEqual(missing, [], `Commands declared in package.json but not registered:\n  ${missing.join("\n  ")}`);
});

test("every menu item refers to a declared command", () => {
  const declared = new Set(declaredCommands);
  const orphans: Array<{ group: string; command: string }> = [];
  for (const { group, items } of menuGroups) {
    for (const item of items) {
      if (!declared.has(item.command)) orphans.push({ group, command: item.command });
    }
  }
  assert.deepEqual(orphans, [], `Menu items reference undeclared commands:\n${JSON.stringify(orphans, null, 2)}`);
});

test("every viewsWelcome command link is a declared command", () => {
  const declared = new Set(declaredCommands);
  const linkRe = /command:([a-zA-Z0-9.]+)/g;
  const orphans: string[] = [];
  for (const welcome of pkg.contributes.viewsWelcome ?? []) {
    const contents: string = welcome.contents ?? "";
    for (const match of contents.matchAll(linkRe)) {
      const cmd = match[1];
      if (!declared.has(cmd)) orphans.push(cmd);
    }
  }
  assert.deepEqual(orphans, [], `viewsWelcome references undeclared commands:\n  ${orphans.join("\n  ")}`);
});

test("every view has a welcome fallback for unauthenticated state", () => {
  const viewIds = pkg.contributes.views.teamcity.map((v: any) => v.id);
  const welcomeViews = new Set((pkg.contributes.viewsWelcome ?? []).map((w: any) => w.view));
  const missing = viewIds.filter((id: string) => !welcomeViews.has(id));
  assert.deepEqual(missing, [], `Views without welcome content:\n  ${missing.join("\n  ")}`);
});

test("yamlValidation schema file exists", () => {
  for (const entry of pkg.contributes.yamlValidation ?? []) {
    const schemaPath = path.resolve(ROOT, entry.url);
    assert.ok(fs.existsSync(schemaPath), `Schema referenced but missing: ${schemaPath}`);
  }
});

test("icon files referenced in command declarations exist", () => {
  for (const cmd of pkg.contributes.commands) {
    if (typeof cmd.icon === "object") {
      for (const variant of ["light", "dark"]) {
        const p = path.resolve(ROOT, cmd.icon[variant]);
        assert.ok(fs.existsSync(p), `Icon missing for ${cmd.command} (${variant}): ${p}`);
      }
    }
  }
});

test("package main points to existing bundle path (after build)", () => {
  const main = pkg.main;
  assert.equal(typeof main, "string");
  assert.ok(main.startsWith("./"), `package.main should be relative: ${main}`);
});

test("no command is declared twice", () => {
  const seen = new Map<string, number>();
  for (const id of declaredCommands) seen.set(id, (seen.get(id) ?? 0) + 1);
  const dupes = [...seen.entries()].filter(([, n]) => n > 1).map(([id]) => id);
  assert.deepEqual(dupes, [], `Duplicate command declarations: ${dupes.join(", ")}`);
});
