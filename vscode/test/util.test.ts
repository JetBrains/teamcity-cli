import { test } from "node:test";
import assert from "node:assert/strict";
import { nodeId, browserPathFor } from "../src/util";

test("nodeId: reads direct keys", () => {
  assert.equal(nodeId({ runId: "42" }, "runId"), "42");
  assert.equal(nodeId({ id: 7 }, "runId", "id"), "7");
  assert.equal(nodeId({ agentName: "mac" }, "agentName"), "mac");
});

test("nodeId: reads from nested data field", () => {
  assert.equal(nodeId({ data: { id: "Build_1" } }, "id"), "Build_1");
  assert.equal(nodeId({ data: { jobId: "J" } }, "jobId"), "J");
});

test("nodeId: returns undefined when no key matches", () => {
  assert.equal(nodeId({ foo: "bar" }, "runId", "id"), undefined);
  assert.equal(nodeId(undefined, "id"), undefined);
});

test("nodeId: tries keys in order", () => {
  assert.equal(nodeId({ runId: "a", id: "b" }, "runId", "id"), "a");
  assert.equal(nodeId({ id: "b" }, "runId", "id"), "b");
});

test("nodeId: coerces to string", () => {
  assert.equal(nodeId({ id: 42 }, "id"), "42");
  assert.equal(nodeId({ id: 0 }, "id"), "0");
});

test("browserPathFor: build (has buildTypeId)", () => {
  assert.equal(browserPathFor({ id: 123, buildTypeId: "CI_Build" }), "/build/123");
});

test("browserPathFor: queued build", () => {
  assert.equal(browserPathFor({ id: 99, buildTypeId: "CI_Build", state: "queued" }), "/build/99");
});

test("browserPathFor: pipeline tree node", () => {
  assert.equal(browserPathFor({ kind: "pipeline", data: { id: "CI_Root" } }), "/pipeline/CI_Root");
});

test("browserPathFor: job tree node uses parent pipelineId", () => {
  assert.equal(
    browserPathFor({ kind: "job", data: { id: "test_macos", pipelineId: "CI_Root" } }),
    "/pipeline/CI_Root",
  );
});

test("browserPathFor: agent", () => {
  assert.equal(browserPathFor({ id: 5, name: "agent1", connected: true }), "/agent/5");
  assert.equal(browserPathFor({ id: 5, name: "agent1", connected: false }), "/agent/5");
});

test("browserPathFor: fallback to buildConfiguration", () => {
  assert.equal(browserPathFor({ id: "BT_1" }), "/buildConfiguration/BT_1");
});

test("browserPathFor: undefined for unknown shapes", () => {
  assert.equal(browserPathFor(undefined), undefined);
  assert.equal(browserPathFor({}), undefined);
});
