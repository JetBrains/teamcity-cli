---
name: teamcity-cli
description: Use when working with TeamCity CI/CD or when user provides a TeamCity build URL. Use `tc` CLI for builds, logs, jobs, queues, agents, and pipelines.
---

# TeamCity CLI (`tc`)

Interact with TeamCity CI/CD servers using the `tc` command-line tool.

## Quick Start

```bash
tc auth status                    # Check authentication
tc run list --status failure      # Find failed builds
tc run log <id> --failed          # View failed build log
```

## Before Running Commands

**Do not guess flags or syntax.** Only use flags in the [Command Reference](references/commands.md). If unsure, run `tc <command> --help` first. If a command doesn't support what you need, fall back to `tc api /app/rest/...`.

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
| API       | `tc api <endpoint>` — raw REST API access                                             |

## Quick Workflows

**Investigate failure:** `tc run list --status failure` → `tc run log <id> --failed` → `tc run tests <id> --failed`
**From a URL:** Extract build ID from `https://host/buildConfiguration/ConfigId/12345` → `tc run view 12345`
**Start build:** `tc run start <job-id> --branch <branch> --watch`
**Find jobs:** `tc project list` → `tc job list --project <id>`

## References

- [Command Reference](references/commands.md) - All commands and flags
- [Workflows](references/workflows.md) - URL handling, failure investigation, artifacts, agents, and more
- [Output Formats](references/output.md) - JSON, plain text, scripting
