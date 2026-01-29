---
name: teamcity-cli
description: Use when working with TeamCity CI/CD. Use `tc` CLI for builds, logs, jobs, queues, and pipelines.
---

# TeamCity CLI (`tc`)

Interact with TeamCity CI/CD servers using the `tc` command-line tool.

## Quick Start

```bash
tc auth status                    # Check authentication
tc run list --status failure      # Find failed builds
tc run log <id> --failed          # View failed build log
```

## Core Commands

| Task           | Command                 |
|----------------|-------------------------|
| List builds    | `tc run list`           |
| View log       | `tc run log <id>`       |
| Start build    | `tc run start <job-id>` |
| Watch build    | `tc run watch <id>`     |
| List jobs      | `tc job list`           |
| List projects  | `tc project list`       |
| View queue     | `tc queue list`         |
| List agents    | `tc agent list`         |
| List pools     | `tc pool list`          |
| Raw API        | `tc api <endpoint>`     |

## Common Workflows

**Investigate failure:** `tc run list --status failure` → `tc run log <id> --failed` → `tc run tests <id> --failed`

**Start build:** `tc run start <job-id> --branch <branch> --watch`

**Find jobs:** `tc project list` → `tc job list --project <id>`

## References

- [Command Reference](references/commands.md) - All commands and flags
- [Workflows](references/workflows.md) - Detailed workflow examples
- [Output Formats](references/output.md) - JSON, plain text, scripting
