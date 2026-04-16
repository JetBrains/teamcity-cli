import { test } from "node:test";
import assert from "node:assert/strict";
import { backoffDelay, pickServer, argsFor } from "../src/util";

// ---------- backoffDelay ----------

test("backoffDelay: no backoff on 0 failures", () => {
  assert.equal(backoffDelay(30_000, 0), 30_000);
});

test("backoffDelay: exponential growth", () => {
  assert.equal(backoffDelay(1000, 1), 2000);
  assert.equal(backoffDelay(1000, 2), 4000);
  assert.equal(backoffDelay(1000, 3), 8000);
  assert.equal(backoffDelay(1000, 4), 16000);
});

test("backoffDelay: capped at max", () => {
  assert.equal(backoffDelay(30_000, 20), 120_000);
  assert.equal(backoffDelay(1000, 100, 5000), 5000);
});

test("backoffDelay: negative fail count treated as zero", () => {
  assert.equal(backoffDelay(1000, -5), 1000);
});

// ---------- pickServer ----------

type S = { server: string; status: string; user?: { username: string } };

test("pickServer: prefers configured default", () => {
  const servers: S[] = [
    { server: "https://a.example.com", status: "authenticated" },
    { server: "https://b.example.com", status: "authenticated" },
  ];
  const p = pickServer(servers, "https://b.example.com");
  assert.equal(p?.server, "https://b.example.com");
});

test("pickServer: falls back to cli.teamcity.com when default not authed", () => {
  const servers: S[] = [
    { server: "https://other.example.com", status: "authenticated" },
    { server: "https://cli.teamcity.com", status: "authenticated" },
  ];
  const p = pickServer(servers, "https://missing.example.com");
  assert.equal(p?.server, "https://cli.teamcity.com");
});

test("pickServer: falls back to first authed when neither preferred is there", () => {
  const servers: S[] = [
    { server: "https://nonauth.example.com", status: "expired" },
    { server: "https://first.example.com", status: "authenticated" },
    { server: "https://second.example.com", status: "authenticated" },
  ];
  const p = pickServer(servers, "https://missing.example.com");
  assert.equal(p?.server, "https://first.example.com");
});

test("pickServer: returns undefined when no servers are authenticated", () => {
  const servers: S[] = [
    { server: "https://a.example.com", status: "expired" },
    { server: "https://b.example.com", status: "unknown" },
  ];
  assert.equal(pickServer(servers, "https://a.example.com"), undefined);
});

test("pickServer: returns undefined for empty input", () => {
  assert.equal(pickServer([], "https://cli.teamcity.com"), undefined);
});

// ---------- argsFor ----------

test("argsFor.login / loginGuest include server", () => {
  assert.deepEqual(argsFor.login("https://srv"), ["auth", "login", "-s", "https://srv"]);
  assert.deepEqual(argsFor.loginGuest("https://srv"), ["auth", "login", "--guest", "-s", "https://srv"]);
});

test("argsFor.logout / authStatus are stable", () => {
  assert.deepEqual(argsFor.logout(), ["auth", "logout"]);
  assert.deepEqual(argsFor.authStatus(), ["auth", "status", "--json"]);
});

test("argsFor.runList with and without status filter", () => {
  assert.deepEqual(argsFor.runList(), ["run", "list", "--json"]);
  assert.deepEqual(argsFor.runList(""), ["run", "list", "--json"]);
  assert.deepEqual(argsFor.runList("failure"), ["run", "list", "--json", "--status", "failure"]);
});

test("argsFor.runListBranch uses @this by default", () => {
  assert.deepEqual(argsFor.runListBranch(), ["run", "list", "--branch", "@this", "--limit", "1", "--json"]);
  assert.deepEqual(argsFor.runListBranch("main"), ["run", "list", "--branch", "main", "--limit", "1", "--json"]);
});

test("argsFor.runListFavorites", () => {
  assert.deepEqual(argsFor.runListFavorites(), ["run", "list", "--favorites", "--json"]);
});

test("argsFor list commands for each noun", () => {
  assert.deepEqual(argsFor.queueList(), ["queue", "list", "--json"]);
  assert.deepEqual(argsFor.agentList(), ["agent", "list", "--json"]);
  assert.deepEqual(argsFor.pipelineList(), ["pipeline", "list", "--json"]);
  assert.deepEqual(argsFor.pipelineView("CI_Root"), ["pipeline", "view", "CI_Root", "--json"]);
});

test("argsFor.triggerRun with and without job id", () => {
  assert.deepEqual(argsFor.triggerRun("Build_1"), ["run", "start", "Build_1", "--watch"]);
  assert.deepEqual(argsFor.triggerRun(undefined), ["run", "start"]);
});

test("argsFor.remoteRun with and without job id", () => {
  assert.deepEqual(argsFor.remoteRun("Build_1"), ["run", "start", "Build_1", "--local-changes"]);
  assert.deepEqual(argsFor.remoteRun(undefined), ["run", "start", "--local-changes"]);
});

test("argsFor.viewLog / viewFailedTests include id", () => {
  assert.deepEqual(argsFor.viewLog("42"), ["run", "log", "42", "--raw"]);
  assert.deepEqual(argsFor.viewFailedTests("42"), ["run", "tests", "42", "--failed", "--json"]);
});

test("argsFor pipeline lifecycle", () => {
  assert.deepEqual(argsFor.pipelineCreate(), ["pipeline", "create"]);
  assert.deepEqual(argsFor.pipelineValidate(), ["pipeline", "validate"]);
  assert.deepEqual(argsFor.pipelinePush(), ["pipeline", "push"]);
});
