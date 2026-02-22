[//]: # (title: AI Agent Integration)

<show-structure for="chapter" depth="2"/>

TeamCity CLI includes a built-in skill that teaches AI coding agents how to use `teamcity` commands for common TeamCity workflows. The skill follows the [Agent Skills specification](https://agentskills.io), so it works with any compatible agent (for example, Claude Code, Cursor, and others).

## Installing the skill

Install the skill for all detected AI agents:

```Shell
teamcity skill install
```

The command auto-detects which AI coding agents are installed on your system and configures the skill for each one. If your agent is not auto-detected, pass `--agent` to target it explicitly.

<img src="skill-install.gif" alt="Installing the AI agent skill" border-effect="rounded"/>

### Install for specific agents

Target one or more specific agents:

```Shell
teamcity skill install --agent claude-code
teamcity skill install --agent claude-code --agent cursor
```

### Project-level installation

Install the skill for the current project only, rather than globally:

```Shell
teamcity skill install --project
```

## Updating the skill

Update the skill to the latest version bundled with your current `teamcity` release:

```Shell
teamcity skill update
```

The command skips the update if the installed version already matches the bundled version.

Target specific agents or install at the project level:

```Shell
teamcity skill update --agent claude-code
teamcity skill update --project
```

## Removing the skill

Remove the skill from AI coding agents:

```Shell
teamcity skill remove
```

Target specific agents or remove from the project level:

```Shell
teamcity skill remove --agent claude-code
teamcity skill remove --project
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

If you use Claude Code, you can also install the TeamCity skill directly through the plugin system:

```Shell
/plugin marketplace add JetBrains/teamcity-cli
/plugin install teamcity-cli@teamcity-cli
```

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="installation">
        <a href="teamcity-cli-get-started.md">Getting started with TeamCity CLI</a>
    </category>
</seealso>
