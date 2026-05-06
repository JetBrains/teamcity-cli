[//]: # (title: TeamCity CLI)

TeamCity CLI is the official command-line interface for TeamCity. Type `teamcity` in your terminal to start builds, tail logs, manage agents and queues, and more.

<img src="showcase.gif" alt="TeamCity CLI in action" border-effect="rounded"/>

> TeamCity CLI is an open-source tool developed and maintained by TeamCity developers. It is not bundled with TeamCity server installers or Docker images. The project is [available on GitHub](https://github.com/JetBrains/teamcity-cli) under the Apache-2.0 license.
>
{style="note"}

> TeamCity CLI supports TeamCity server versions 2020.1 and later and is distributed as a standalone binary with no additional runtime dependencies. Some features may require newer TeamCity versions (for example, 2024.04 or later).
>
{style="tip"}

## Features

- **Stay in your terminal** — Start builds, view logs, manage queues — no browser needed.
- **Remote agent access** — Shell into any build agent with `teamcity agent term`, or run commands with `teamcity agent exec`.
- **Real-time logs** — Stream build output as it happens with `teamcity run watch --logs`.
- **Scriptable** — `--json` and `--plain` output for pipelines, plus direct REST API access via `teamcity api`.
- **Multi-server support** — Authenticate with and switch between multiple TeamCity instances.
- **AI agent ready** — Built-in [skill](https://agentskills.io) for Claude Code, Cursor, and other AI coding agents — just run `teamcity skill install`.

## Installing TeamCity CLI {id="installing"}

<snippet id="install-cli">

<tabs>
<tab title="macOS and Linux">

**Homebrew (recommended):**

```bash
brew install jetbrains/utils/teamcity
```

**Install script:**

```bash
curl -fsSL https://jb.gg/tc/install | bash
```

</tab>
<tab title="Windows">

**Winget (recommended):**

```PowerShell
winget install JetBrains.TeamCityCLI
```

**PowerShell (install script):**

```PowerShell
irm https://jb.gg/tc/install.ps1 | iex
```

</tab>
</tabs>

</snippet>

**npm (cross-platform):**

```bash
npm install -g @jetbrains/teamcity-cli
```

For other methods (Scoop, Chocolatey, deb/rpm packages, building from source), see the [getting started guide](teamcity-cli-get-started.md).

## Quick start

```Shell
# Authenticate with your TeamCity server
teamcity auth login

# List recent builds
teamcity run list --limit 10

# Start a build and watch it run
teamcity run start MyProject_Build --branch main --watch

# View logs from the latest build of a job
teamcity run log --job MyProject_Build

# Check what's in the queue
teamcity queue list
```

For a full walkthrough, see [Getting started with TeamCity CLI](teamcity-cli-get-started.md).

## What you can do

<table>
<tr>
<td>

Topic

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

[Getting started](teamcity-cli-get-started.md)

</td>
<td>

Install, authenticate, and run your first commands

</td>
</tr>
<tr>
<td>

[Authentication](teamcity-cli-authentication.md)

</td>
<td>

Token storage, guest access, multi-server setup, and CI/CD usage

</td>
</tr>
<tr>
<td>

[Configuration](teamcity-cli-configuration.md)

</td>
<td>

Configuration file, environment variables, and shell completion

</td>
</tr>
<tr>
<td>

[Linking a repository](teamcity-cli-linking.md)

</td>
<td>

Bind a repo (or monorepo subdirectories) to TeamCity projects via `teamcity.toml`

</td>
</tr>
<tr>
<td>

[Managing runs](teamcity-cli-managing-runs.md)

</td>
<td>

Start, monitor, and manage builds

</td>
</tr>
<tr>
<td>

[Managing pipelines](teamcity-cli-managing-pipelines.md)

</td>
<td>

Author, validate, and sync `.teamcity.yml` pipelines

</td>
</tr>
<tr>
<td>

[Managing jobs](teamcity-cli-managing-jobs.md)

</td>
<td>

View and configure build configurations

</td>
</tr>
<tr>
<td>

[Managing projects](teamcity-cli-managing-projects.md)

</td>
<td>

Browse projects, manage VCS roots, parameters, and versioned settings

</td>
</tr>
<tr>
<td>

[Managing the build queue](teamcity-cli-managing-build-queue.md)

</td>
<td>

Inspect queued builds, control priority, and approve builds

</td>
</tr>
<tr>
<td>

[Managing agents](teamcity-cli-managing-agents.md)

</td>
<td>

Monitor, control, and access build agents remotely

</td>
</tr>
<tr>
<td>

[Managing agent pools](teamcity-cli-managing-agent-pools.md)

</td>
<td>

Assign projects and agents to pools

</td>
</tr>
<tr>
<td>

[Managing cloud agents](teamcity-cli-managing-cloud-agents.md)

</td>
<td>

Manage cloud profiles, images, and ephemeral agent instances

</td>
</tr>
<tr>
<td>

[REST API access](teamcity-cli-rest-api-access.md)

</td>
<td>

Make authenticated API requests from the command line

</td>
</tr>
<tr>
<td>

[Aliases](teamcity-cli-aliases.md)

</td>
<td>

Create custom command shortcuts

</td>
</tr>
<tr>
<td>

[Scripting and automation](teamcity-cli-scripting.md)

</td>
<td>

JSON output, plain text mode, and CI/CD integration

</td>
</tr>
<tr>
<td>

[Command reference](teamcity-cli-commands.md)

</td>
<td>

Quick reference for all available commands and flags

</td>
</tr>
<tr>
<td>

[AI agent integration](teamcity-cli-ai-agent-integration.md)

</td>
<td>

Install the TeamCity skill for AI coding agents

</td>
</tr>
<tr>
<td>

[Glossary](teamcity-cli-glossary.md)

</td>
<td>

Definitions of CLI terms and how they map to TeamCity concepts

</td>
</tr>
<tr>
<td>

[Anonymous usage statistics](teamcity-cli-analytics.md)

</td>
<td>

What the CLI sends to JetBrains and how to opt out

</td>
</tr>
</table>

<seealso>
    <category ref="installation">
        <a href="teamcity-cli-get-started.md">Getting started with TeamCity CLI</a>
    </category>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
</seealso>
