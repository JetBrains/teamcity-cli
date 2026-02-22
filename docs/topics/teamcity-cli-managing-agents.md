[//]: # (title: Managing Agents)

<show-structure for="chapter" depth="2"/>

Build agents are machines that run your builds. The `teamcity agent` command group lets you list and inspect agents, enable or disable them, manage authorization, execute remote commands, open interactive shell sessions, and request reboots.

## Listing agents

View all registered build agents:

```Shell
teamcity agent list
```

<img src="agent-list.gif" alt="Listing build agents" border-effect="rounded"/>

### Filtering

```Shell
# Only connected agents
teamcity agent list --connected

# Only enabled agents
teamcity agent list --enabled

# Only authorized agents
teamcity agent list --authorized

# Agents in a specific pool
teamcity agent list --pool Default

# Combine filters
teamcity agent list --connected --enabled --pool Default
```

Limit results and output as JSON:

```Shell
teamcity agent list --limit 20
teamcity agent list --json
teamcity agent list --json=id,name,connected,enabled,pool.name
```

### agent list flags

<table>
<tr>
<td>

Flag

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`--connected`

</td>
<td>

Show only connected agents

</td>
</tr>
<tr>
<td>

`--enabled`

</td>
<td>

Show only enabled agents

</td>
</tr>
<tr>
<td>

`--authorized`

</td>
<td>

Show only authorized agents

</td>
</tr>
<tr>
<td>

`-p`, `--pool`

</td>
<td>

Filter by agent pool name

</td>
</tr>
<tr>
<td>

`-n`, `--limit`

</td>
<td>

Maximum number of agents to display

</td>
</tr>
<tr>
<td>

`--json`

</td>
<td>

Output as JSON. Use `--json=` to list available fields, `--json=f1,f2` for specific fields.

</td>
</tr>
</table>

## Viewing agent details

View details of a specific agent by ID or name:

```Shell
teamcity agent view 1
teamcity agent view Agent-Linux-01
teamcity agent view Agent-Linux-01 --web
teamcity agent view 1 --json
```

<img src="agent-view.gif" alt="Viewing agent details" border-effect="rounded"/>

## Enabling and disabling agents

Disable an agent to prevent it from picking up new builds:

```Shell
teamcity agent disable 1
teamcity agent disable Agent-Linux-01
```

Enable an agent to allow it to run builds again:

```Shell
teamcity agent enable 1
teamcity agent enable Agent-Linux-01
```

> Disabling an agent does not stop builds that are already running on it. New builds will not be assigned to the agent until it is re-enabled.
>
{style="note"}

## Authorizing and deauthorizing agents

Authorize a newly connected agent to allow it to run builds:

```Shell
teamcity agent authorize 1
teamcity agent authorize Agent-Linux-01
```

Deauthorize an agent to revoke its permission to connect:

```Shell
teamcity agent deauthorize 1
teamcity agent deauthorize Agent-Linux-01
```

> An unauthorized agent can connect to the server but cannot run builds. You need to authorize it before it can be used.
>
{style="note"}

## Moving agents between pools

Move an agent to a different agent pool:

```Shell
teamcity agent move 1 0
teamcity agent move Agent-Linux-01 2
```

The first argument is the agent (by ID or name), and the second argument is the target pool ID.

## Viewing compatible jobs

List the build configurations that an agent can run:

```Shell
teamcity agent jobs 1
teamcity agent jobs Agent-Linux-01
```

Show incompatible jobs with the reasons why they cannot run on the agent:

```Shell
teamcity agent jobs Agent-Linux-01 --incompatible
teamcity agent jobs 1 --json
```

<img src="agent-jobs.gif" alt="Viewing compatible and incompatible jobs" border-effect="rounded"/>

## Executing remote commands

Run a command on a build agent and return the output:

```Shell
teamcity agent exec 1 "ls -la"
teamcity agent exec Agent-Linux-01 "cat /etc/os-release"
```

<img src="agent-exec.gif" alt="Executing remote commands on an agent" border-effect="rounded"/>

Set a timeout for long-running commands:

```Shell
teamcity agent exec Agent-Linux-01 --timeout 10m -- long-running-script.sh
```

> Remote command execution requires appropriate permissions on the TeamCity server.
>
{style="note"}

## Interactive shell sessions

Open an interactive terminal session to a build agent:

```Shell
teamcity agent term 1
teamcity agent term Agent-Linux-01
```

This establishes a WebSocket connection to the agent and provides a shell where you can run commands directly on the agent machine. The session ends when you type `exit` or press `Ctrl+D`.

<img src="agent-term.gif" alt="Interactive terminal session on an agent" border-effect="rounded"/>

> The `agent term` command requires the build agent to support the terminal feature and the server to have it enabled.
>
{style="note"}

## Rebooting agents

Request a reboot of a build agent:

```Shell
teamcity agent reboot 1
teamcity agent reboot Agent-Linux-01
```

Wait for the current build to finish before rebooting:

```Shell
teamcity agent reboot Agent-Linux-01 --after-build
```

Skip the confirmation prompt:

```Shell
teamcity agent reboot Agent-Linux-01 --yes
```

> Local agents (running on the same machine as the server) cannot be rebooted through this command.
>
{style="warning"}

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
        <a href="teamcity-cli-managing-agent-pools.md">Managing agent pools</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-managing-runs.md">Managing runs</a>
        <a href="teamcity-cli-managing-build-queue.md">Managing the build queue</a>
    </category>
</seealso>
