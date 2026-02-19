---
name: teamcity-cli
version: "0.5.0"
author: JetBrains
description: Use when working with TeamCity CI/CD or when user provides a TeamCity build URL. Use `teamcity` CLI for builds, logs, jobs, queues, agents, and pipelines.
---

# TeamCity CLI (`teamcity`)

## Quick Start

```bash
teamcity auth status                    # Check authentication
teamcity run list --status failure      # Find failed builds
teamcity run log <id> --failed          # View failed build log
```

## Before Running Commands

**Do not guess subcommands, flags, or syntax.** Only use commands and flags documented in the [Command Reference](references/commands.md) or shown by `teamcity <command> --help`. If a command doesn't support what you need, fall back to `teamcity api /app/rest/...`.

**Terminology:** There is no `build`, `pipeline`, or `config` subcommand. Builds are **runs** (`teamcity run`). Build configurations are **jobs** (`teamcity job`).

## Core Commands

| Area      | Commands                                                                              |
|-----------|---------------------------------------------------------------------------------------|
| Builds    | `run list`, `view`, `start`, `watch`, `log`, `cancel`, `restart`, `tests`, `changes`  |
| Artifacts | `run artifacts`, `run download`                                                       |
| Metadata  | `run pin/unpin`, `run tag/untag`, `run comment`                                       |
| Jobs      | `job list`, `view`, `pause/resume`, `param list/get/set/delete`                       |
| Projects  | `project list`, `view`, `param`, `token put/get`, `settings export/status/validate`   |
| Queue     | `queue list`, `approve`, `remove`, `top`                                              |
| Agents    | `agent list`, `view`, `enable/disable`, `authorize`, `exec`, `term`, `reboot`, `move` |
| Pools     | `pool list`, `view`, `link/unlink`                                                    |
| API       | `teamcity api <endpoint>` — raw REST API access                                             |

## Quick Workflows

**Investigate failure:** `teamcity run list --status failure` → `teamcity run log <id> --failed` → `teamcity run tests <id> --failed`
**From a URL:** Extract build ID from `https://host/buildConfiguration/ConfigId/12345` → `teamcity run view 12345`
**Start build:** `teamcity run start <job-id> --branch <branch> --watch`
**Find jobs:** `teamcity project list` → `teamcity job list --project <id>`

## References

- [Command Reference](references/commands.md) - All commands and flags
- [Workflows](references/workflows.md) - URL handling, failure investigation, artifacts, agents, and more
- [Output Formats](references/output.md) - JSON, plain text, scripting
