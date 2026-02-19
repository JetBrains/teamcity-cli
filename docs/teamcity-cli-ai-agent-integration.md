[//]: # (title: TeamCity CLI AI Agent Integration)

<show-structure for="chapter" depth="2"/>

TeamCity CLI includes a built-in skill that teaches AI coding agents (such as Claude Code and Cursor) how to use `tc` commands for common TeamCity workflows. The skill follows the [Agent Skills specification](https://agentskills.io).

## Installing the skill

Install the skill for all detected AI agents:

```Shell
tc skill install
```

The command auto-detects which AI coding agents are installed on your system and configures the skill for each one.

### Install for specific agents

Target one or more specific agents:

```Shell
tc skill install --agent claude-code
tc skill install --agent claude-code --agent cursor
```

### Project-level installation

Install the skill for the current project only, rather than globally:

```Shell
tc skill install --project
```

## Updating the skill

Update the skill to the latest version bundled with your current `tc` release:

```Shell
tc skill update
```

The command skips the update if the installed version already matches the bundled version.

Target specific agents or install at the project level:

```Shell
tc skill update --agent claude-code
tc skill update --project
```

## Removing the skill

Remove the skill from AI coding agents:

```Shell
tc skill remove
```

Target specific agents or remove from the project level:

```Shell
tc skill remove --agent claude-code
tc skill remove --project
```

## skill flags

These flags are shared across `skill install`, `skill update`, and `skill remove`:

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

`-a`, `--agent`

</td>
<td>

Target agent(s). Can be repeated. Auto-detects installed agents if omitted.

</td>
</tr>
<tr>
<td>

`--project`

</td>
<td>

Install to the current project instead of globally

</td>
</tr>
</table>

## Alternative installation for Claude Code

You can also install the TeamCity skill in Claude Code directly through the plugin system:

```Shell
/plugin marketplace add JetBrains/teamcity-cli
/plugin install teamcity-cli@teamcity-cli
```

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="installation">
        <a href="install-teamcity-cli.md">Installing TeamCity CLI</a>
    </category>
</seealso>
