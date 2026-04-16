import "./_setup";
import { test } from "node:test";
import assert from "node:assert/strict";

// eslint-disable-next-line @typescript-eslint/no-var-requires
const { PipelineCodeLensProvider } = require("../src/providers/pipelineCodeLens");

function mkDoc(yaml: string) {
  const lines = yaml.split("\n");
  return {
    lineCount: lines.length,
    lineAt: (i: number) => ({ text: lines[i] ?? "" }),
  };
}

function lens(yaml: string) {
  const provider = new PipelineCodeLensProvider();
  return provider.provideCodeLenses(mkDoc(yaml));
}

test("always emits Validate and Push lenses at the top", () => {
  const lenses = lens("image: alpine\n");
  const cmds = lenses.map((l: any) => l.command.command);
  assert.ok(cmds.includes("teamcity.validatePipeline"));
  assert.ok(cmds.includes("teamcity.pushPipeline"));
});

test("emits a Run Job lens for each job", () => {
  const yaml = [
    "name: CI",
    "jobs:",
    "  build:",
    "    script: make",
    "  test:",
    "    script: make test",
    "  deploy:",
    "    script: make deploy",
  ].join("\n");

  const lenses = lens(yaml);
  const runLenses = lenses.filter((l: any) => l.command.command === "teamcity.triggerRun");
  assert.equal(runLenses.length, 3);

  const jobIds = runLenses.map((l: any) => l.command.arguments[0].jobId);
  assert.deepEqual(jobIds.sort(), ["build", "deploy", "test"]);
});

test("ignores non-job top-level keys after jobs:", () => {
  const yaml = [
    "jobs:",
    "  compile:",
    "    script: make",
    "environment:",
    "  VAR: value",
  ].join("\n");

  const runLenses = lens(yaml).filter((l: any) => l.command.command === "teamcity.triggerRun");
  assert.equal(runLenses.length, 1);
  assert.equal(runLenses[0].command.arguments[0].jobId, "compile");
});

test("handles empty jobs section", () => {
  const yaml = "name: empty\njobs:\n";
  const runLenses = lens(yaml).filter((l: any) => l.command.command === "teamcity.triggerRun");
  assert.equal(runLenses.length, 0);
});

test("ignores nested keys (indented > 2 spaces)", () => {
  const yaml = [
    "jobs:",
    "  outer:",
    "    steps:",
    "      - run: echo hi",
    "    env:",
    "      X: 1",
  ].join("\n");

  const runLenses = lens(yaml).filter((l: any) => l.command.command === "teamcity.triggerRun");
  assert.equal(runLenses.length, 1);
  assert.equal(runLenses[0].command.arguments[0].jobId, "outer");
});

test("supports hyphenated and underscore job names", () => {
  const yaml = [
    "jobs:",
    "  build_mac:",
    "    script: make",
    "  build-linux:",
    "    script: make",
    "  test_123:",
    "    script: make",
  ].join("\n");

  const runLenses = lens(yaml).filter((l: any) => l.command.command === "teamcity.triggerRun");
  assert.equal(runLenses.length, 3);
});

test("returns a lens count that includes the two top-level lenses", () => {
  const yaml = "jobs:\n  one:\n    script: true\n";
  const all = lens(yaml);
  assert.equal(all.length, 3); // Validate + Push + 1 Run Job
});
