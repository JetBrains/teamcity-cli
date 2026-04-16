import "./_setup";
import { test } from "node:test";
import assert from "node:assert/strict";

// eslint-disable-next-line @typescript-eslint/no-var-requires
const { buildItem, buildIcon } = require("../src/providers/buildItem");

type B = {
  id: number; buildTypeId: string; number?: string;
  state?: string; status?: string; percentageComplete?: number;
  buildType?: { name: string }; statusText?: string;
  branchName?: string; agent?: { id: number; name: string };
};

const mk = (p: Partial<B> = {}): B => ({
  id: 1, buildTypeId: "Cfg", number: "1", state: "finished", status: "SUCCESS", ...p,
});

// ---------- buildIcon ----------

test("buildIcon: running → spinning sync", () => {
  const icon = buildIcon(mk({ state: "running" }));
  assert.equal(icon.id, "sync~spin");
  assert.equal(icon.color.id, "charts.green");
});

test("buildIcon: success → check", () => {
  const icon = buildIcon(mk({ state: "finished", status: "SUCCESS" }));
  assert.equal(icon.id, "check");
  assert.equal(icon.color.id, "charts.green");
});

test("buildIcon: failure → error", () => {
  const icon = buildIcon(mk({ state: "finished", status: "FAILURE" }));
  assert.equal(icon.id, "error");
  assert.equal(icon.color.id, "charts.red");
});

test("buildIcon: unknown state → circle-outline without color", () => {
  const icon = buildIcon(mk({ state: "queued", status: undefined }));
  assert.equal(icon.id, "circle-outline");
});

// ---------- buildItem ----------

test("buildItem: label is #number", () => {
  const item = buildItem(mk({ id: 100, number: "1234" }));
  assert.equal(item.label, "#1234");
});

test("buildItem: label falls back to id when number missing", () => {
  const item = buildItem(mk({ id: 77, number: undefined }));
  assert.equal(item.label, "#77");
});

test("buildItem: description is build type name", () => {
  const item = buildItem(mk({ buildType: { name: "Compile & Test" } }));
  assert.equal(item.description, "Compile & Test");
});

test("buildItem: description falls back to buildTypeId", () => {
  const item = buildItem(mk({ buildTypeId: "CI_Compile", buildType: undefined }));
  assert.equal(item.description, "CI_Compile");
});

test("buildItem: running adds percentage to description", () => {
  const item = buildItem(mk({
    state: "running",
    percentageComplete: 42,
    buildType: { name: "Run" },
  }));
  assert.equal(item.description, "Run (42%)");
});

test("buildItem: tooltip includes statusText, branch, and agent", () => {
  const item = buildItem(mk({
    statusText: "Tests passed",
    branchName: "main",
    agent: { id: 1, name: "agent-42" },
  }));
  assert.match(String(item.tooltip), /Tests passed/);
  assert.match(String(item.tooltip), /Branch: main/);
  assert.match(String(item.tooltip), /Agent: agent-42/);
});

test("buildItem: running gets running contextValue", () => {
  const item = buildItem(mk({ state: "running" }));
  assert.equal(item.contextValue, "run.running.webUrl");
});

test("buildItem: finished gets finished contextValue", () => {
  const item = buildItem(mk({ state: "finished" }));
  assert.equal(item.contextValue, "run.finished.webUrl");
});

test("buildItem: click opens viewLog with runId", () => {
  const item = buildItem(mk({ id: 500 }));
  assert.equal(item.command.command, "teamcity.viewLog");
  assert.deepEqual(item.command.arguments, [{ runId: "500" }]);
});

test("buildItem: carries runId and jobId on the item", () => {
  const item = buildItem(mk({ id: 500, buildTypeId: "CI_Build" }));
  assert.equal((item as any).runId, "500");
  assert.equal((item as any).jobId, "CI_Build");
});
