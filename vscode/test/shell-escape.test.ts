import "./_setup";
import { test } from "node:test";
import assert from "node:assert/strict";

// eslint-disable-next-line @typescript-eslint/no-var-requires
const { shellEscape, buildTerminalCommand } = require("../src/cli/runner");

// ---------- shellEscape ----------

test("empty string becomes ''", () => {
  assert.equal(shellEscape(""), "''");
});

test("plain alphanumeric passes through", () => {
  assert.equal(shellEscape("pipeline"), "pipeline");
  assert.equal(shellEscape("CI_Build_1"), "CI_Build_1");
});

test("paths and URLs pass through", () => {
  assert.equal(shellEscape("/Users/tv/go/bin/teamcity"), "/Users/tv/go/bin/teamcity");
  assert.equal(shellEscape("https://cli.teamcity.com"), "https://cli.teamcity.com");
  assert.equal(shellEscape("--no-color"), "--no-color");
  assert.equal(shellEscape("-s"), "-s");
});

test("branch-style paths with slashes pass through", () => {
  assert.equal(shellEscape("refs/heads/main"), "refs/heads/main");
  assert.equal(shellEscape("feature-foo"), "feature-foo");
});

test("spaces force quoting", () => {
  assert.equal(shellEscape("hello world"), "'hello world'");
});

test("special shell chars force quoting", () => {
  assert.equal(shellEscape("a;b"), "'a;b'");
  assert.equal(shellEscape("a|b"), "'a|b'");
  assert.equal(shellEscape("a&b"), "'a&b'");
  assert.equal(shellEscape("a$b"), "'a$b'");
  assert.equal(shellEscape("a>b"), "'a>b'");
  assert.equal(shellEscape("a*b"), "'a*b'");
});

test("embedded single quotes are escaped safely", () => {
  // bash-compatible escape: close-quote, literal quote, reopen-quote.
  assert.equal(shellEscape("O'Reilly"), "'O'\\''Reilly'");
  assert.equal(shellEscape("'"), "''\\'''");
});

// ---------- buildTerminalCommand ----------

test("buildTerminalCommand always injects --no-color after the binary", () => {
  const cmd = buildTerminalCommand("/usr/local/bin/teamcity", ["pipeline", "create"]);
  assert.equal(cmd, "/usr/local/bin/teamcity --no-color pipeline create");
});

test("buildTerminalCommand preserves argument order and quoting", () => {
  const cmd = buildTerminalCommand("/bin/tc", ["auth", "login", "-s", "https://srv.example.com"]);
  assert.equal(cmd, "/bin/tc --no-color auth login -s https://srv.example.com");
});

test("buildTerminalCommand quotes a binary path containing spaces", () => {
  const cmd = buildTerminalCommand("/Users/me/Bin With Space/teamcity", ["auth", "logout"]);
  assert.ok(cmd.startsWith("'/Users/me/Bin With Space/teamcity'"));
});

test("buildTerminalCommand quotes args that would otherwise be dangerous", () => {
  const cmd = buildTerminalCommand("/bin/tc", ["run", "start", "weird$name"]);
  assert.equal(cmd, "/bin/tc --no-color run start 'weird$name'");
});
