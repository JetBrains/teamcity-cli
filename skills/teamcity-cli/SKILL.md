---
name: teamcity-cli
version: "0.8.3"
author: JetBrains
description: Use when working with TeamCity CI/CD or when user provides a TeamCity build URL. Use `teamcity` CLI for builds, logs, jobs, queues, and agents.
---

# TeamCity CLI (`teamcity`)

## Quick Start

```bash
teamcity auth status                    # Check authentication
teamcity run list --status failure      # Find failed builds
teamcity run log <id> --failed --raw    # Full failure diagnostics
```

**Do not guess flags or syntax.** Run `teamcity help --json` to get the full machine-readable command schema, or `teamcity help --json <command>` for a specific command. The schema includes flag types, defaults, enum constraints, and available JSON output fields. Fall back to the [Command Reference](references/commands.md) for quick lookups, or `teamcity api '/app/rest/...'` when no specific command exists. Builds are **runs** (`teamcity run`), build configurations are **jobs** (`teamcity job`).
**Never use `--count`.** The TeamCity CLI uses `--limit` (or `-n`) for list-style limits. `--count` is not a valid substitute.

## Important

- **Build logs:** Always use `teamcity run log <id> --raw` to avoid interactive terminal formatting. Dump the output to a temp file so you can re-read it as needed.
- **Starting builds:** Always use `teamcity run start <job-id> --watch` to wait until the build finishes before proceeding.
- **Branch names:** Always verify you are passing the correct branch when using `teamcity run start <job-id> --branch <branch>`. Do not guess branch names.
- **Local changes and Kotlin DSL:** When using `teamcity run start <job-id> --local-changes`, local changes to Kotlin DSL (`.teamcity/`) are **not** included in the remote run. Always push Kotlin DSL changes before running the build.

## Schema Introspection

Use `help --json` to discover commands programmatically — always up-to-date, no guessing:

```bash
teamcity help --json                # full command tree with all flags and metadata
teamcity help --json run list       # single command: flags, types, defaults, enums, JSON fields
```

The schema returns typed defaults, enum constraints, available `--json` output fields, and inherited global flags. Prefer this over `--help` when building automated pipelines or validating flag combinations.

## Core Commands

`run` (builds), `job` (build configs), `project`, `queue`, `agent`, `pool`, `api` (raw REST). Use `teamcity help --json` for the full tree with all subcommands and flags.

## Quick Workflows

**Investigate failure:** `teamcity run list --status failure` → `teamcity run log <id> --failed --raw` → `teamcity run tests <id> --failed`
**From a URL:** Extract build ID from `https://host/buildConfiguration/ConfigId/12345` → `teamcity run view 12345`
**Start build:** `teamcity run start <job-id> --branch <branch> --watch`
**Find jobs:** `teamcity project list` → `teamcity job list --project <id>`
**Validate Kotlin DSL:** `teamcity project settings validate` (not `mvn compile` — see [Workflows](references/workflows.md))

## References

- [Command Reference](references/commands.md) - All commands and flags
- [Workflows](references/workflows.md) - URL handling, failure investigation, artifacts, agents, and more
- [Output Formats](references/output.md) - JSON, plain text, scripting
