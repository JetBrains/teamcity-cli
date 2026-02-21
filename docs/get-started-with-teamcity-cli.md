[//]: # (title: Getting started with TeamCity CLI)

<show-structure for="chapter" depth="2"/>

This guide walks you through authenticating with a TeamCity server and running your first commands. It assumes you have already [installed TeamCity CLI](install-teamcity-cli.md).

## Authenticate with your server

Run the login command to connect to your TeamCity instance:

```Shell
teamcity auth login
```

The CLI will:

1. Prompt for your TeamCity server URL.
2. Open your browser to generate an access token.
3. Validate and store the token securely.

Tokens are stored in your system keyring (macOS Keychain, GNOME Keyring, or Windows Credential Manager) when available.

> If your system does not have a keyring, the CLI falls back to storing the token in the configuration file. You can force this behavior with `--insecure-storage`.
>
{style="note"}

### Guest access

If the server has guest access enabled, you can log in without a token:

```Shell
teamcity auth login --guest
```

Guest access provides read-only access to the server.

### Verify authentication

To verify that authentication succeeded:

```Shell
teamcity auth status
```

## List recent builds

View the most recent builds across all projects:

```Shell
teamcity run list
```

Filter by project, job, branch, or status:

```Shell
# Builds from a specific job
teamcity run list --job MyProject_Build

# Only failures from the last 24 hours
teamcity run list --status failure --since 24h

# Builds on a specific branch
teamcity run list --branch main --limit 10
```

## Start a build

Trigger a new build by specifying a job ID:

```Shell
teamcity run start MyProject_Build
```

Add `--watch` to follow the build in real time:

```Shell
teamcity run start MyProject_Build --branch main --watch
```

The `--watch` flag displays a live progress view that updates until the build completes.

## View build logs

View the log output from a specific build:

```Shell
teamcity run log 12345
```

Or get the latest log for a job:

```Shell
teamcity run log --job MyProject_Build
```

The log output opens in a pager by default. Use `/` to search, `n`/`N` to navigate matches, and `q` to quit. Pass `--raw` to bypass the pager.

## Check the build queue

See what builds are waiting to run:

```Shell
teamcity queue list
```

## View build agents

List all registered build agents and their status:

```Shell
teamcity agent list
```

Filter to show only connected agents:

```Shell
teamcity agent list --connected
```

## Open a build in the browser

Most view commands support a `--web` flag that opens the corresponding page in your browser:

```Shell
teamcity run view 12345 --web
teamcity job view MyProject_Build --web
teamcity project view MyProject --web
```

## Enable shell completion

Set up tab completion for your shell:

```Shell
# Bash
teamcity completion bash > /etc/bash_completion.d/teamcity

# Zsh
teamcity completion zsh > "${fpath[1]}/_teamcity"

# Fish
teamcity completion fish > ~/.config/fish/completions/teamcity.fish

# PowerShell
teamcity completion powershell > teamcity.ps1
```

## Next steps

- Learn about [authentication methods](teamcity-cli-authentication.md) including guest access, multi-server setup, and CI/CD usage.
- Explore the full [command reference](teamcity-cli-commands.md).
- Set up [aliases](teamcity-cli-aliases.md) for frequently used commands.
- Configure [JSON output](teamcity-cli-scripting.md) for scripting and automation.

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
        <a href="teamcity-cli-configuration.md">Configuration</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-managing-runs.md">Managing runs</a>
        <a href="teamcity-cli-aliases.md">Aliases</a>
        <a href="teamcity-cli-scripting.md">Scripting and automation</a>
    </category>
</seealso>
