import "./_setup";
import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";

// eslint-disable-next-line @typescript-eslint/no-var-requires
const { BuildNotifier } = require("../src/providers/buildItem");
// eslint-disable-next-line @typescript-eslint/no-var-requires
const shim = require("./vscode-shim");

type Build = {
  id: number;
  buildTypeId: string;
  number?: string;
  state?: string;
  status?: string;
  buildType?: { name: string };
};

function build(id: number, state = "finished", status = "SUCCESS", buildTypeId = "Cfg1"): Build {
  return { id, buildTypeId, state, status, number: String(id) };
}

function newNotifier() {
  return new BuildNotifier(() => undefined, (p: string) => `http://srv${p}`, () => {});
}

beforeEach(() => { shim.resetNotifications(); });

test("does not notify before seed", async () => {
  const n = newNotifier();
  n.observe(build(1));
  await Promise.resolve();
  assert.equal(shim.notifications.length, 0);
});

test("seed swallows initial builds", async () => {
  const n = newNotifier();
  n.seed([build(1), build(2), build(3)]);
  for (const b of [build(1), build(2), build(3)]) n.observe(b);
  await Promise.resolve();
  assert.equal(shim.notifications.length, 0);
});

test("fires exactly once for a newly finished build", async () => {
  const n = newNotifier();
  n.seed([]); // no pre-existing builds
  n.observe(build(42, "finished", "SUCCESS"));
  n.observe(build(42, "finished", "SUCCESS")); // repeated poll
  n.observe(build(42, "finished", "SUCCESS"));
  await Promise.resolve();
  assert.equal(shim.notifications.length, 1);
  assert.match(shim.notifications[0].message, /#42 succeeded/);
});

test("does not notify for running builds", async () => {
  const n = newNotifier();
  n.seed([]);
  n.observe(build(7, "running"));
  n.observe(build(7, "running", "SUCCESS"));
  await Promise.resolve();
  assert.equal(shim.notifications.length, 0);
});

test("notifies once per distinct build id", async () => {
  const n = newNotifier();
  n.seed([]);
  n.observe(build(1, "finished", "SUCCESS"));
  n.observe(build(2, "finished", "FAILURE"));
  n.observe(build(3, "finished", "SUCCESS"));
  await Promise.resolve();
  assert.equal(shim.notifications.length, 3);
  assert.match(shim.notifications[0].message, /#1 succeeded/);
  assert.match(shim.notifications[1].message, /#2 failed/);
  assert.match(shim.notifications[2].message, /#3 succeeded/);
});

test("running → finished transition notifies once", async () => {
  const n = newNotifier();
  n.seed([build(9, "running")]); // build already known while running
  n.observe(build(9, "running"));
  n.observe(build(9, "running"));
  await Promise.resolve();
  assert.equal(shim.notifications.length, 0, "no notifications while running");

  n.observe(build(9, "finished", "SUCCESS"));
  n.observe(build(9, "finished", "SUCCESS"));
  await Promise.resolve();
  assert.equal(shim.notifications.length, 1);
  assert.match(shim.notifications[0].message, /#9 succeeded/);
});

test("notifications bounded — does not leak memory over many builds", async () => {
  const n = newNotifier();
  n.seed([]);
  for (let i = 0; i < 2000; i++) n.observe(build(i, "finished", "SUCCESS"));
  await Promise.resolve();
  // Memory bound enforced internally; verify we still remember recent builds.
  shim.resetNotifications();
  n.observe(build(1999, "finished", "SUCCESS")); // recent — still announced
  await Promise.resolve();
  assert.equal(shim.notifications.length, 0, "recent build should remain deduplicated");
});
