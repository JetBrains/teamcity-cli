[//]: # (title: AI Agent Skill)

<show-structure for="chapter" depth="2"/>

TeamCity CLI includes a built-in skill that teaches AI coding agents how to use `teamcity` commands for common TeamCity workflows. The skill follows the [Agent Skills specification](https://agentskills.io), so it works with any compatible agent (for example, Claude Code, Cursor, and others).

<tip>

You can also integrate TeamCity with AI agents via the [MCP server](ai-agent-integration.md#TeamCity+MCP).

</tip>

## Listing skills

List the skills bundled with your `teamcity` release, with each one's version and which is installed by default:

```Shell
teamcity skill list
```

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

## Onboarding follow-ups

After the core onboarding pipeline is green, the agent offers applicable
follow-ups one at a time. Remaining GitHub Actions migration includes missing
test matrix variants and preserves existing core jobs. It starts only after
explicit agreement; declining or leaving the offer unanswered starts nothing.

The agent loads the instructions in the same session:

```Shell
teamcity skill guide teamcity-migrate-remaining-gha
```

A separate installation or agent restart is not required. To continue later,
ask your agent to run this command and identify the existing server, project,
and pipeline. The migration report records expected test matrix combinations,
mapped TeamCity jobs, run evidence, and unresolved gaps.

Each hook is a separate embedded skill. A new or changed hook requires a CLI
release; installation with `skill install` is optional. The decision map runs
after core work, offering one implemented hook in the order red-build,
migrate, feedback, speed-up, deploy. Future hook tickets add their own rows.

After refusal, the agent prints a ready-to-copy request containing the server,
project and pipeline IDs, checkout and report paths, accepted scope, and guide
command. A fresh session recovers context from the report and current pipeline
settings before changing anything.

### Hook analytics

Agents report actual offers and explicit answers:

```Shell
teamcity skill hook-event teamcity-migrate-remaining-gha offered
teamcity skill hook-event teamcity-migrate-remaining-gha accepted --json
teamcity skill hook-event teamcity-migrate-remaining-gha declined
```

Use `accepted` only after explicit consent, `declined` only after explicit
refusal. No answer emits neither response event. A direct continuation request
records acceptance without a fabricated offer. Each observation is reported
once. Loading a guide records `guide.viewed` separately and does not imply consent.

`hook-event` records telemetry only: it does not authorize or run a hook. It
accepts only registered bundled hooks and the three listed event values.
The JSON result contains `skill` and `event`; it acknowledges command processing,
not server receipt. Analytics respects opt-out and remains best effort.
Hook events contain the skill name, without project or conversation details.

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
<tr>
<td>

`--all`

</td>
<td>

Apply to every bundled skill instead of only the default

</td>
</tr>
</table>

## Read-only mode for AI agents

When giving AI agents access to TeamCity, you may want to prevent them from triggering builds, canceling runs, or modifying configuration. Set `TEAMCITY_RO=1` to restrict the CLI to read-only operations:

```Shell
export TEAMCITY_RO=1
teamcity skill install --agent claude-code
```

In read-only mode, the agent can list builds, view logs, inspect failures, and query the API, but any command that would modify data is blocked. You can also set `ro: true` per server in the [configuration file](teamcity-cli-configuration.md#Configuration+file).

## Sandbox compatibility

An agent's command sandbox, such as the Codex sandbox (on by default) or Claude Code and Junie `/sandbox`, hides the system keyring and blocks network access. Inside it, the CLI can't read the token saved by `teamcity auth login` or reach your TeamCity server, and reports the token as missing. Allowing the server domain in the sandbox's network settings isn't enough: the keyring stays hidden.

Run `teamcity` commands outside the sandbox:

- **Claude Code**: add `teamcity *` to `sandbox.excludedCommands` in your settings. Claude Code still asks before each excluded command, and keeps a command with a pipe, a redirect, or `$(…)` sandboxed.
- **Codex**: approve the agent's request to run `teamcity` outside the sandbox. To allow it in all sessions without asking, approve the `teamcity` prefix for good, or add `prefix_rule(pattern=["teamcity"], decision="allow")` to a `.rules` file in `~/.codex/rules/`.
- **Junie**: turn sandbox mode off with `/sandbox`.

When Claude Code or Codex run commands in a sandbox, `teamcity onboard` asks before it starts them, and sets this up when you agree. When the CLI detects an agent sandbox, its error messages say so instead of suggesting `TEAMCITY_TOKEN`.

The CLI also loads TLS root certificates from PEM bundle files and respects HTTP proxy settings, so it works where platform certificate stores and direct network access are restricted.

## Alternative installation for Claude Code

If you use Claude Code, you can install the `teamcity` plugin, which bundles the TeamCity skills from the [JetBrains/teamcity-skills](https://github.com/JetBrains/teamcity-skills) repository:

```Shell
/plugin marketplace add JetBrains/teamcity-skills
/plugin install teamcity@teamcity
```

The plugin provides the skills only. Install the `teamcity` CLI separately.

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="installation">
        <a href="teamcity-cli-get-started.md">Getting started with TeamCity CLI</a>
    </category>
</seealso>
