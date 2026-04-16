import "./_setup";
import { test } from "node:test";
import assert from "node:assert/strict";

// eslint-disable-next-line @typescript-eslint/no-var-requires
const { BACKGROUND_ACTIONS, TERMINAL_ACTIONS } = require("../src/commands");

type BA = { id: string; args: (id: string) => string[]; success: string; refresh?: boolean; idKeys?: string[] };
type TA = { id: string; name: string; args: (id?: string) => string[]; idKeys?: string[] };

const bgById = new Map<string, BA>((BACKGROUND_ACTIONS as BA[]).map((a) => [a.id, a]));
const termById = new Map<string, TA>((TERMINAL_ACTIONS as TA[]).map((a) => [a.id, a]));

// ---------- BACKGROUND_ACTIONS ----------

test("cancelRun posts `run cancel <id> -y` and refreshes", () => {
  const a = bgById.get("teamcity.cancelRun")!;
  assert.deepEqual(a.args("42"), ["run", "cancel", "42", "-y"]);
  assert.equal(a.refresh, true);
});

test("pinRun / unpinRun do not refresh", () => {
  assert.deepEqual(bgById.get("teamcity.pinRun")!.args("42"), ["run", "pin", "42"]);
  assert.deepEqual(bgById.get("teamcity.unpinRun")!.args("42"), ["run", "unpin", "42"]);
  assert.notEqual(bgById.get("teamcity.pinRun")!.refresh, true);
  assert.notEqual(bgById.get("teamcity.unpinRun")!.refresh, true);
});

test("queue actions", () => {
  assert.deepEqual(bgById.get("teamcity.approveQueued")!.args("7"), ["queue", "approve", "7"]);
  assert.deepEqual(bgById.get("teamcity.removeFromQueue")!.args("7"), ["queue", "remove", "7", "-y"]);
  assert.deepEqual(bgById.get("teamcity.moveToTop")!.args("7"), ["queue", "top", "7"]);
});

test("agent enable/disable use agentId key", () => {
  const enable = bgById.get("teamcity.enableAgent")!;
  const disable = bgById.get("teamcity.disableAgent")!;
  assert.deepEqual(enable.args("MyAgent"), ["agent", "enable", "MyAgent"]);
  assert.deepEqual(disable.args("MyAgent"), ["agent", "disable", "MyAgent"]);
  assert.deepEqual(enable.idKeys, ["agentId", "id"]);
  assert.deepEqual(disable.idKeys, ["agentId", "id"]);
});

test("every BACKGROUND_ACTION has a non-empty success message", () => {
  for (const a of BACKGROUND_ACTIONS as BA[]) {
    assert.ok(a.success && a.success.length > 0, `empty success for ${a.id}`);
  }
});

// ---------- TERMINAL_ACTIONS ----------

test("watchRun: run watch --logs", () => {
  assert.deepEqual(termById.get("teamcity.watchRun")!.args("99"), ["run", "watch", "99", "--logs"]);
});

test("restartRun: run restart --watch", () => {
  assert.deepEqual(termById.get("teamcity.restartRun")!.args("99"), ["run", "restart", "99", "--watch"]);
});

test("downloadArtifacts: run download", () => {
  assert.deepEqual(termById.get("teamcity.downloadArtifacts")!.args("99"), ["run", "download", "99"]);
});

test("agentTerminal: uses agentName key first", () => {
  const t = termById.get("teamcity.agentTerminal")!;
  assert.deepEqual(t.args("mac-mini-1"), ["agent", "term", "mac-mini-1"]);
  assert.deepEqual(t.idKeys, ["agentName", "agentId", "name"]);
});

test("every TERMINAL_ACTION has a name", () => {
  for (const a of TERMINAL_ACTIONS as TA[]) {
    assert.ok(a.name && a.name.length > 0, `empty terminal name for ${a.id}`);
  }
});

// ---------- Cross-check: every declared action ID is a declared vscode command ----------

test("every action table entry matches a command declared in package.json", () => {
  const pkg = require("../package.json");
  const declared = new Set<string>(pkg.contributes.commands.map((c: any) => c.command));
  for (const a of [...BACKGROUND_ACTIONS, ...TERMINAL_ACTIONS] as Array<{ id: string }>) {
    assert.ok(declared.has(a.id), `action ${a.id} has no package.json command entry`);
  }
});
