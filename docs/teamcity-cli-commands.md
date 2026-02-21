[//]: # (title: TeamCity CLI Command Reference)

<show-structure for="chapter" depth="2"/>

This page provides a quick reference for all available TeamCity CLI commands. Each command group links to a detailed page with full descriptions, flags, and examples.

## Authentication

Manage server authentication. See [Authentication](teamcity-cli-authentication.md) for details.

<table>
<tr>
<td>

Command

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`teamcity auth login`

</td>
<td>

Authenticate with a TeamCity server

</td>
</tr>
<tr>
<td>

`teamcity auth logout`

</td>
<td>

Log out from the current server

</td>
</tr>
<tr>
<td>

`teamcity auth status`

</td>
<td>

Show authentication status

</td>
</tr>
</table>

## Runs

Start, monitor, and manage builds. See [Managing runs](teamcity-cli-managing-runs.md) for details.

<table>
<tr>
<td>

Command

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`teamcity run list`

</td>
<td>

List recent builds

</td>
</tr>
<tr>
<td>

`teamcity run start`

</td>
<td>

Start a new build

</td>
</tr>
<tr>
<td>

`teamcity run view`

</td>
<td>

View build details

</td>
</tr>
<tr>
<td>

`teamcity run watch`

</td>
<td>

Watch a build in real time

</td>
</tr>
<tr>
<td>

`teamcity run log`

</td>
<td>

View build log output

</td>
</tr>
<tr>
<td>

`teamcity run cancel`

</td>
<td>

Cancel a running or queued build

</td>
</tr>
<tr>
<td>

`teamcity run restart`

</td>
<td>

Restart a build with the same configuration

</td>
</tr>
<tr>
<td>

`teamcity run download`

</td>
<td>

Download build artifacts

</td>
</tr>
<tr>
<td>

`teamcity run artifacts`

</td>
<td>

List artifacts without downloading

</td>
</tr>
<tr>
<td>

`teamcity run tests`

</td>
<td>

Show test results

</td>
</tr>
<tr>
<td>

`teamcity run changes`

</td>
<td>

Show VCS commits included in a build

</td>
</tr>
<tr>
<td>

`teamcity run pin`

</td>
<td>

Pin a build to prevent cleanup

</td>
</tr>
<tr>
<td>

`teamcity run unpin`

</td>
<td>

Unpin a build

</td>
</tr>
<tr>
<td>

`teamcity run tag`

</td>
<td>

Add tags to a build

</td>
</tr>
<tr>
<td>

`teamcity run untag`

</td>
<td>

Remove tags from a build

</td>
</tr>
<tr>
<td>

`teamcity run comment`

</td>
<td>

Set, view, or delete a build comment

</td>
</tr>
</table>

## Jobs

View and configure build configurations. See [Managing jobs](teamcity-cli-managing-jobs.md) for details.

<table>
<tr>
<td>

Command

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`teamcity job list`

</td>
<td>

List build configurations

</td>
</tr>
<tr>
<td>

`teamcity job view`

</td>
<td>

View job details

</td>
</tr>
<tr>
<td>

`teamcity job pause`

</td>
<td>

Pause a job (prevent new builds)

</td>
</tr>
<tr>
<td>

`teamcity job resume`

</td>
<td>

Resume a paused job

</td>
</tr>
<tr>
<td>

`teamcity job param list`

</td>
<td>

List job parameters

</td>
</tr>
<tr>
<td>

`teamcity job param get`

</td>
<td>

Get a specific parameter value

</td>
</tr>
<tr>
<td>

`teamcity job param set`

</td>
<td>

Set a parameter value

</td>
</tr>
<tr>
<td>

`teamcity job param delete`

</td>
<td>

Delete a parameter

</td>
</tr>
</table>

## Projects

Browse projects and manage parameters and settings. See [Managing projects](teamcity-cli-managing-projects.md) for details.

<table>
<tr>
<td>

Command

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`teamcity project list`

</td>
<td>

List projects

</td>
</tr>
<tr>
<td>

`teamcity project view`

</td>
<td>

View project details

</td>
</tr>
<tr>
<td>

`teamcity project param list`

</td>
<td>

List project parameters

</td>
</tr>
<tr>
<td>

`teamcity project param get`

</td>
<td>

Get a specific parameter value

</td>
</tr>
<tr>
<td>

`teamcity project param set`

</td>
<td>

Set a parameter value

</td>
</tr>
<tr>
<td>

`teamcity project param delete`

</td>
<td>

Delete a parameter

</td>
</tr>
<tr>
<td>

`teamcity project token put`

</td>
<td>

Store a secure token for versioned settings

</td>
</tr>
<tr>
<td>

`teamcity project token get`

</td>
<td>

Retrieve a secure token value

</td>
</tr>
<tr>
<td>

`teamcity project settings export`

</td>
<td>

Export project settings as a ZIP archive

</td>
</tr>
<tr>
<td>

`teamcity project settings status`

</td>
<td>

Show versioned settings sync status

</td>
</tr>
<tr>
<td>

`teamcity project settings validate`

</td>
<td>

Validate Kotlin DSL configuration

</td>
</tr>
</table>

## Queue

Manage the build queue. See [Managing the build queue](teamcity-cli-managing-build-queue.md) for details.

<table>
<tr>
<td>

Command

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`teamcity queue list`

</td>
<td>

List queued builds

</td>
</tr>
<tr>
<td>

`teamcity queue approve`

</td>
<td>

Approve a build that requires manual approval

</td>
</tr>
<tr>
<td>

`teamcity queue remove`

</td>
<td>

Remove a build from the queue

</td>
</tr>
<tr>
<td>

`teamcity queue top`

</td>
<td>

Move a build to the top of the queue

</td>
</tr>
</table>

## Agents

Monitor and control build agents. See [Managing agents](teamcity-cli-managing-agents.md) for details.

<table>
<tr>
<td>

Command

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`teamcity agent list`

</td>
<td>

List build agents

</td>
</tr>
<tr>
<td>

`teamcity agent view`

</td>
<td>

View agent details

</td>
</tr>
<tr>
<td>

`teamcity agent enable`

</td>
<td>

Enable an agent for builds

</td>
</tr>
<tr>
<td>

`teamcity agent disable`

</td>
<td>

Disable an agent

</td>
</tr>
<tr>
<td>

`teamcity agent authorize`

</td>
<td>

Authorize an agent to connect

</td>
</tr>
<tr>
<td>

`teamcity agent deauthorize`

</td>
<td>

Revoke agent authorization

</td>
</tr>
<tr>
<td>

`teamcity agent move`

</td>
<td>

Move an agent to a different pool

</td>
</tr>
<tr>
<td>

`teamcity agent jobs`

</td>
<td>

List compatible or incompatible jobs

</td>
</tr>
<tr>
<td>

`teamcity agent exec`

</td>
<td>

Execute a command on an agent

</td>
</tr>
<tr>
<td>

`teamcity agent term`

</td>
<td>

Open an interactive shell to an agent

</td>
</tr>
<tr>
<td>

`teamcity agent reboot`

</td>
<td>

Request an agent reboot

</td>
</tr>
</table>

## Agent pools

Manage agent pool assignments. See [Managing agent pools](teamcity-cli-managing-agent-pools.md) for details.

<table>
<tr>
<td>

Command

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`teamcity pool list`

</td>
<td>

List agent pools

</td>
</tr>
<tr>
<td>

`teamcity pool view`

</td>
<td>

View pool details

</td>
</tr>
<tr>
<td>

`teamcity pool link`

</td>
<td>

Link a project to a pool

</td>
</tr>
<tr>
<td>

`teamcity pool unlink`

</td>
<td>

Unlink a project from a pool

</td>
</tr>
</table>

## API

Make raw REST API requests. See [REST API access](teamcity-cli-rest-api-access.md) for details.

<table>
<tr>
<td>

Command

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`teamcity api <endpoint>`

</td>
<td>

Make an authenticated HTTP request to the TeamCity REST API

</td>
</tr>
</table>

## Aliases

Create custom command shortcuts. See [Aliases](teamcity-cli-aliases.md) for details.

<table>
<tr>
<td>

Command

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`teamcity alias set`

</td>
<td>

Create a command alias

</td>
</tr>
<tr>
<td>

`teamcity alias list`

</td>
<td>

List configured aliases

</td>
</tr>
<tr>
<td>

`teamcity alias delete`

</td>
<td>

Delete an alias

</td>
</tr>
</table>

## Skills

Manage AI agent integration. See [AI agent integration](teamcity-cli-ai-agent-integration.md) for details.

<table>
<tr>
<td>

Command

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`teamcity skill install`

</td>
<td>

Install the TeamCity skill for AI coding agents

</td>
</tr>
<tr>
<td>

`teamcity skill update`

</td>
<td>

Update the skill to the latest version

</td>
</tr>
<tr>
<td>

`teamcity skill remove`

</td>
<td>

Remove the skill

</td>
</tr>
</table>

<seealso>
    <category ref="installation">
        <a href="get-started-with-teamcity-cli.md">Getting started with TeamCity CLI</a>
    </category>
    <category ref="reference">
        <a href="teamcity-cli-configuration.md">Configuration</a>
    </category>
</seealso>
