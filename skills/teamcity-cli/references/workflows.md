# Common Workflows

Each workflow is a self-contained doc under [`workflows/`](workflows/). Read the
one that matches the task instead of loading everything.

## Diagnosing & fixing failures

- [Investigate a build failure](workflows/investigate-failure.md) — walk a FAILURE from overview to root cause; build-chain drill-down; failure classification decision tree.
- [Fix a failure & monitor until green](workflows/fix-and-verify.md) — diagnose → classify → fix (code / Kotlin DSL / pipeline YAML / server) → verify with `--local-changes` → loop until green.
- [Test reliability analysis](workflows/test-reliability.md) — spot flaky tests by cross-referencing failures across builds.

## Running & inspecting builds

- [Inspect a build from a TeamCity URL](workflows/inspect-from-url.md) — parse a TC URL and map it to the right `teamcity` command.
- [Start, monitor & personal builds](workflows/run-builds.md) — `run start`/`watch`, branches, params, `--local-changes`, `--dry-run`.
- [Build artifacts](workflows/artifacts.md) — list and download artifacts.
- [Build metadata & the queue](workflows/build-metadata-and-queue.md) — pin/unpin, tag, comment; manage the build queue.

## Projects, jobs & config

- [Find jobs and projects](workflows/find-jobs-projects.md) — list/create projects, list/view/search jobs.
- [Job & project parameters](workflows/parameters.md) — list/get/set/delete params, secure params.
- [Validate Kotlin DSL locally](workflows/kotlin-dsl.md) — the only correct DSL validation step.
- [Project settings & secure tokens](workflows/project-settings-and-tokens.md) — export/status of versioned settings; store/retrieve secret tokens.

## Connections, VCS & agents

- [Project connections & VCS roots](workflows/connections-and-vcs.md) — GitHub App and Docker connections; create/inspect VCS roots.
- [Agents & agent pools](workflows/agents.md) — list/view/enable/authorize/move/reboot agents, remote `term`/`exec`, pools.

## Pipelines

- [Working with pipelines](workflows/pipelines.md) — YAML-first pipelines: list, view, create, validate, pull/push, delete.

## Reference

- [Tips & troubleshooting](workflows/troubleshooting.md) — `--json`/`jq`, `teamcity api` escape hatch, env vars, and a symptom→cause→action table.
