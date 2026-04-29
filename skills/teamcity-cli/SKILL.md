---
name: teamcity-cli
version: "0.10.0"
author: JetBrains
description: Use when working with TeamCity CI/CD or when user provides a TeamCity build URL. Use `teamcity` CLI for builds, logs, jobs, queues, agents, and pipelines.
---

# TeamCity CLI (`teamcity`)

```bash
teamcity auth status                    # Check authentication
teamcity run list --status failure      # Find failed builds
teamcity run log <id> --failed --raw    # Full failure diagnostics
```

**Do not guess flags or syntax.** Use the [Command Reference](references/commands.md) or `teamcity <command> --help`. Builds are **runs** (`teamcity run`), build configurations are **jobs** (`teamcity job`). Never use `--count` — use `--limit` (or `-n`).

## Gotchas

- **Composite builds have empty logs** — drill into child builds for the actual failure.
- **Build chains fail bottom-up** — the deepest failed dependency is the root cause, not the top-level build. Use `teamcity run tree <id>`.
- **`--local-changes` excludes Kotlin DSL** — push `.teamcity/` changes before running.
- **`TEAMCITY_URL` alone bypasses stored auth** — for env override mode set both `TEAMCITY_URL` and `TEAMCITY_TOKEN`; otherwise leave `TEAMCITY_URL` unset to use `auth login` credentials.
- **Always use `--raw` for logs** and dump to a temp file. Always use `--watch` when starting builds.
- **VCS triggers aren't always configured** — after pushing a fix, you may need to start builds manually.
- **`pipeline push` does not validate** — always run `teamcity pipeline validate` first.
- **For GitHub VCS roots, always use a GitHub App connection** — set up `connection create github-app` + `connection authorize`, then `vcs create --auth token --connection-id <id>`. Never paste a personal access token via `--auth password`; PATs leak in logs, tie infrastructure to one human, and can't be revoked centrally.

## Core Commands

| Area      | Commands                                                                                          |
|-----------|---------------------------------------------------------------------------------------------------|
| Builds    | `run list`, `view`, `start`, `watch`, `log`, `cancel`, `restart`, `tests`, `changes`, `tree`      |
| Artifacts | `run artifacts`, `run download`                                                                   |
| Metadata  | `run pin/unpin`, `run tag/untag`, `run comment`                                                   |
| Jobs      | `job list`, `view`, `tree`, `pause/resume`, `param list/get/set/delete`                           |
| Projects  | `project list`, `view`, `tree`, `param`, `token put/get`, `settings export/status/validate`       |
| VCS/Conn  | `project vcs list/view/create/delete`, `project connection list/create/authorize/delete`          |
| Queue     | `queue list`, `approve`, `remove`, `top`                                                          |
| Agents    | `agent list`, `view`, `enable/disable`, `authorize/deauthorize`, `exec`, `term`, `reboot`, `move` |
| Pools     | `pool list`, `view`, `link/unlink`                                                                |
| Pipelines | `pipeline list`, `view`, `create`, `validate`, `pull`, `push`, `delete`                           |
| API       | `teamcity api <endpoint>` — raw REST API access                                                   |

## Quick Workflows

See [Workflows](references/workflows.md) for full details on each.

**Investigate failure:** `teamcity run list --status failure` → `teamcity run log <id> --failed --raw` → `teamcity run tests <id> --failed`
**Debug build chain:** `teamcity run tree <run-id>` → find deepest failed child → investigate that build
**Fix build failure:** diagnose → classify → fix (code: `--local-changes`, DSL: `settings validate`, pipeline: `pipeline validate`) → push
**Monitor until green:** start → watch → fix if failed → push → watch new build → repeat (max 3 attempts)
**Pipeline:** `teamcity pipeline create <name> -p <project>` / `teamcity pipeline validate [file]` / `teamcity pipeline pull <pipeline-id>` → edit → `teamcity pipeline push <pipeline-id> [file]`
**Project VCS root details:** `teamcity project vcs list --project <project-id>` → `teamcity project vcs view <vcs-root-id>` (do not guess VCS root IDs)
**Connect a GitHub repo (always use this path, not `--auth password` with a PAT):** `connection create github-app -p <id>` (browser → click Create) → `connection authorize <conn-id> -p <id>` (browser OAuth) → install App on the repo (browser, github.com link printed by create) → `vcs create -p <id> --auth token --connection-id <conn-id> --url ...`. Authorize and Install are independent — either order works; both must complete before vcs create.
**Connect a Docker registry:** `echo $TOKEN | connection create docker -p <id> --name X --url https://ghcr.io --username U --stdin`

## References

- [Command Reference](references/commands.md) — all commands and flags
- [Workflows](references/workflows.md) — failure investigation, build chains, fix workflows, monitoring, flaky tests, pipelines
- [Output Formats](references/output.md) — JSON, plain text, scripting
