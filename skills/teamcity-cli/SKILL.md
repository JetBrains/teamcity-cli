---
name: teamcity-cli
version: 0.10.1
description: Use when working with TeamCity CI/CD or when a user provides a TeamCity build URL — drives the `teamcity` CLI for builds, logs, jobs, queues, agents, pools, projects, and pipelines.
---

# TeamCity CLI (`teamcity`)

## Quick Start

```bash
teamcity auth status                    # Check authentication
teamcity run list --status failure      # Find failed builds
teamcity run log <id> --failed --raw    # Full failure diagnostics
```

**Do not guess flags or syntax.** Use the [command reference](references/commands.md) or `teamcity <command> --help`. Builds are **runs** (`teamcity run`); build configurations are **jobs** (`teamcity job`). Never use `--count` — use `--limit` (or `-n`).

## Gotchas

- **Composite builds have empty logs** — drill into child builds for the actual failure.
- **Build chains fail bottom-up** — deepest failed dependency is the root cause. Use `teamcity run tree <id>`.
- **`--local-changes` excludes Kotlin DSL** — push `.teamcity/` changes before running.
- **`TEAMCITY_URL` alone bypasses stored auth** — set both `TEAMCITY_URL` and `TEAMCITY_TOKEN`, or leave unset.
- **Logs**: use `--raw` and dump to a temp file. **Builds**: use `--watch` when starting them.
- **VCS triggers aren't always wired up** — after pushing a fix you may need to start builds manually.
- **`pipeline push` does not validate** — always `teamcity pipeline validate` first.
- **GitHub VCS roots: use a GitHub App connection.** Never paste a PAT via `--auth password`. See [workflows](references/workflows.md).

## Core Commands

| Area      | Commands                                                                                          |
|-----------|---------------------------------------------------------------------------------------------------|
| Auth      | `auth login`, `logout`, `status`                                                                  |
| Builds    | `run list`, `view`, `start`, `watch`, `log`, `cancel`, `restart`, `tests`, `changes`, `tree`      |
| Artifacts | `run artifacts`, `run download`                                                                   |
| Metadata  | `run pin/unpin`, `run tag/untag`, `run comment`                                                   |
| Jobs      | `job list`, `view`, `tree`, `pause/resume`, `param list/get/set/delete`                           |
| Projects  | `project list`, `view`, `create`, `tree`, `param`, `token put/get`, `settings export/status`      |
| VCS/Conn  | `project vcs list/view/create/delete`, `project connection list/create/authorize/delete`          |
| Queue     | `queue list`, `approve`, `remove`, `top`                                                          |
| Agents    | `agent list`, `view`, `enable/disable`, `authorize/deauthorize`, `exec`, `term`, `reboot`, `move` |
| Pools     | `pool list`, `view`, `link/unlink`                                                                |
| Pipelines | `pipeline list`, `view`, `create`, `validate`, `pull`, `push`, `schema`, `delete`                 |
| API       | `teamcity api <endpoint>` — raw REST access                                                       |
| Link      | `teamcity link` — bind repo via `teamcity.toml`                                                   |

## Quick Workflows

See [Workflows](references/workflows.md) for full details on each.

- **Investigate failure**: `run list --status failure` → `run log <id> --failed --raw` → `run tests <id> --failed`
- **Debug build chain**: `run tree <id>` → drill to deepest failed child
- **Fix and verify**: edit → push → `run start --watch` (use `--local-changes` for personal builds)
- **Pipeline lifecycle**: `pipeline pull <id>` → edit → `pipeline validate` → `pipeline push <id>`, `pipeline schema` to get the actual schema from the server
- **GitHub VCS**: `connection create github-app` → `connection authorize` → install App on repo → `vcs create --auth token --connection-id <id>`
- **Docker registry**: `echo $TOKEN | connection create docker -p <id> --name X --url https://ghcr.io --username U --stdin`

## References

- [Command reference](references/commands.md) — all commands and flags
- [Workflows](references/workflows.md) — failure investigation, build chains, connections, pipelines
- [Output formats](references/output.md) — JSON, plain text, scripting
