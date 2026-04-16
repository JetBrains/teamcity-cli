import "./_setup";
import { test } from "node:test";
import assert from "node:assert/strict";

// eslint-disable-next-line @typescript-eslint/no-var-requires
const { platformKey, versionMatches } = require("../src/cli/binary");

// ---------- platformKey ----------

test("platformKey: darwin arm64", () => {
  assert.equal(platformKey("darwin", "arm64", "0.9.0"), "teamcity_0.9.0_darwin_arm64.tar.gz");
});

test("platformKey: darwin x64 maps to x86_64", () => {
  assert.equal(platformKey("darwin", "x64", "0.9.0"), "teamcity_0.9.0_darwin_x86_64.tar.gz");
});

test("platformKey: linux x64", () => {
  assert.equal(platformKey("linux", "x64", "0.9.0"), "teamcity_0.9.0_linux_x86_64.tar.gz");
});

test("platformKey: linux arm64", () => {
  assert.equal(platformKey("linux", "arm64", "0.9.0"), "teamcity_0.9.0_linux_arm64.tar.gz");
});

test("platformKey: windows uses zip", () => {
  assert.equal(platformKey("win32", "x64", "0.9.0"), "teamcity_0.9.0_windows_x86_64.zip");
  assert.equal(platformKey("win32", "arm64", "0.9.0"), "teamcity_0.9.0_windows_arm64.zip");
});

test("platformKey: version substituted", () => {
  assert.equal(platformKey("linux", "x64", "1.2.3"), "teamcity_1.2.3_linux_x86_64.tar.gz");
});

// ---------- versionMatches ----------

test("versionMatches: exact match passes", () => {
  assert.equal(versionMatches("teamcity version 0.9.0", "0.9.0"), true);
});

test("versionMatches: trailing newline tolerated", () => {
  assert.equal(versionMatches("teamcity version 0.9.0\n", "0.9.0"), true);
});

test("versionMatches: different version fails", () => {
  assert.equal(versionMatches("teamcity version 0.8.0", "0.9.0"), false);
});

test("versionMatches: dev build fails", () => {
  assert.equal(versionMatches("teamcity version dev", "0.9.0"), false);
});

test("versionMatches: empty output fails", () => {
  assert.equal(versionMatches("", "0.9.0"), false);
});

test("versionMatches: malformed output fails", () => {
  assert.equal(versionMatches("not a version", "0.9.0"), false);
});
