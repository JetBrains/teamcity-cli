[//]: # (title: Managing Agents)

<show-structure for="chapter" depth="2"/>

Build agents are machines that run your builds. The `tc agent` command group lets you list and inspect agents, enable or disable them, manage authorization, execute remote commands, open interactive shell sessions, and request reboots.

## Listing agents

View all registered build agents:

```Shell
tc agent list
```

### Filtering

```Shell
# Only connected agents
tc agent list --connected

# Only enabled agents
tc agent list --enabled

# Only authorized agents
tc agent list --authorized

# Agents in a specific pool
tc agent list --pool Default

# Combine filters
tc agent list --connected --enabled --pool Default
```

Limit results and output as JSON:

```Shell
tc agent list --limit 20
tc agent list --json
tc agent list --json=id,name,connected,enabled,pool.name
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
tc agent view 1
tc agent view Agent-Linux-01
tc agent view Agent-Linux-01 --web
tc agent view 1 --json
```

## Enabling and disabling agents

Disable an agent to prevent it from picking up new builds:

```Shell
tc agent disable 1
tc agent disable Agent-Linux-01
```

Enable an agent to allow it to run builds again:

```Shell
tc agent enable 1
tc agent enable Agent-Linux-01
```

> Disabling an agent does not stop builds that are already running on it. New builds will not be assigned to the agent until it is re-enabled.
>
{style="note"}

## Authorizing and deauthorizing agents

Authorize a newly connected agent to allow it to run builds:

```Shell
tc agent authorize 1
tc agent authorize Agent-Linux-01
```

Deauthorize an agent to revoke its permission to connect:

```Shell
tc agent deauthorize 1
tc agent deauthorize Agent-Linux-01
```

> An unauthorized agent can connect to the server but cannot run builds. You need to authorize it before it can be used.
>
{style="note"}

## Moving agents between pools

Move an agent to a different agent pool:

```Shell
tc agent move 1 0
tc agent move Agent-Linux-01 2
```

The first argument is the agent (by ID or name), and the second argument is the target pool ID.

## Viewing compatible jobs

List the build configurations that an agent can run:

```Shell
tc agent jobs 1
tc agent jobs Agent-Linux-01
```

Show incompatible jobs with the reasons why they cannot run on the agent:

```Shell
tc agent jobs Agent-Linux-01 --incompatible
tc agent jobs 1 --json
```

## Executing remote commands

Run a command on a build agent and return the output:

```Shell
tc agent exec 1 "ls -la"
tc agent exec Agent-Linux-01 "cat /etc/os-release"
```

Set a timeout for long-running commands:

```Shell
tc agent exec Agent-Linux-01 --timeout 10m -- long-running-script.sh
```

> Remote command execution requires appropriate permissions on the TeamCity server.
>
{style="note"}

## Interactive shell sessions

Open an interactive terminal session to a build agent:

```Shell
tc agent term 1
tc agent term Agent-Linux-01
```

This establishes a WebSocket connection to the agent and provides a shell where you can run commands directly on the agent machine. The session ends when you type `exit` or press `Ctrl+D`.

> The `agent term` command requires the build agent to support the terminal feature and the server to have it enabled.
>
{style="note"}

## Rebooting agents

Request a reboot of a build agent:

```Shell
tc agent reboot 1
tc agent reboot Agent-Linux-01
```

Wait for the current build to finish before rebooting:

```Shell
tc agent reboot Agent-Linux-01 --after-build
```

Skip the confirmation prompt:

```Shell
tc agent reboot Agent-Linux-01 --yes
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
