[//]: # (title: TeamCity CLI)

TeamCity CLI (`teamcity`) is a command-line interface for [TeamCity](https://www.jetbrains.com/teamcity/) that allows you to start builds, view logs, manage agents, and control build queues directly from your terminal.

The CLI provides a comprehensive set of commands organized by domain:

- **Authentication** — Log in, switch between servers, manage access tokens
- **Runs** — Start, monitor, and manage builds
- **Jobs** — View and configure build configurations
- **Projects** — Browse projects, manage parameters and versioned settings
- **Queue** — Inspect and manage the build queue
- **Agents** — Monitor, control, and access build agents remotely
- **Pools** — Manage agent pool assignments
- **API** — Make authenticated REST API requests directly

> TeamCity CLI uses the terms "run" and "job" as shorter equivalents of "build" and "build configuration" in TeamCity. All commands map directly to the corresponding TeamCity concepts.

## Terminology mapping

<table>
<tr>
<td>

CLI term

</td>
<td>

TeamCity term

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

run

</td>
<td>

build

</td>
<td>

A single execution of a build configuration

</td>
</tr>
<tr>
<td>

job

</td>
<td>

build configuration

</td>
<td>

A set of instructions for running a build

</td>
</tr>
</table>

## Key features

- **Terminal-first workflow** — Start builds, view logs, manage queues without leaving the terminal
- **Remote agent access** — Open interactive shell sessions to build agents or execute commands remotely
- **Real-time monitoring** — Stream build logs and status updates as they happen
- **Scriptable output** — JSON and plain text output formats for automation and pipelines
- **Multi-server support** — Authenticate with and switch between multiple TeamCity instances
- **AI agent integration** — Built-in skill for AI coding agents such as Claude Code and Cursor

## Getting started

To begin using TeamCity CLI:

1. [Install TeamCity CLI](install-teamcity-cli.md) on your machine.
2. Follow the [quickstart guide](get-started-with-teamcity-cli.md) to authenticate and run your first commands.
3. Explore the [command reference](teamcity-cli-commands.md) for a full list of available commands.

## Learn more

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

[Installing TeamCity CLI](install-teamcity-cli.md)

</td>
<td>

Installation options for macOS, Linux, and Windows

</td>
</tr>
<tr>
<td>

[Getting started with TeamCity CLI](get-started-with-teamcity-cli.md)

</td>
<td>

Authenticate and run your first commands

</td>
</tr>
<tr>
<td>

[Authentication](teamcity-cli-authentication.md)

</td>
<td>

Authentication methods, token storage, and multi-server setup

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

[Command reference](teamcity-cli-commands.md)

</td>
<td>

Quick reference for all available commands

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

Browse projects, manage parameters and settings

</td>
</tr>
<tr>
<td>

[Managing the build queue](teamcity-cli-managing-build-queue.md)

</td>
<td>

Inspect and control the build queue

</td>
</tr>
<tr>
<td>

[Managing agents](teamcity-cli-managing-agents.md)

</td>
<td>

Monitor, control, and access build agents

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

[AI agent integration](teamcity-cli-ai-agent-integration.md)

</td>
<td>

Install the TeamCity skill for AI coding agents

</td>
</tr>
</table>

<seealso>
    <category ref="installation">
        <a href="install-teamcity-cli.md">Installing TeamCity CLI</a>
        <a href="get-started-with-teamcity-cli.md">Getting started with TeamCity CLI</a>
    </category>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
</seealso>
