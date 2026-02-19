[//]: # (title: Getting started with TeamCity CLI)

<show-structure for="chapter" depth="2"/>

This guide walks you through authenticating with a TeamCity server and running your first commands. It assumes you have already [installed TeamCity CLI](install-teamcity-cli.md).

## Authenticate with your server

Run the login command to connect to your TeamCity instance:

```Shell
tc auth login
```

The CLI will:

1. Prompt for your TeamCity server URL.
2. Open your browser to generate an access token.
3. Validate and store the token securely.

Tokens are stored in your system keyring (macOS Keychain, GNOME Keyring, or Windows Credential Manager) when available.

> If your system does not have a keyring, the CLI falls back to storing the token in the configuration file. You can force this behavior with `--insecure-storage`.
>
{style="note"}

To verify that authentication succeeded:

```Shell
tc auth status
```

## List recent builds

View the most recent builds across all projects:

```Shell
tc run list
```

Filter by project, job, branch, or status:

```Shell
# Builds from a specific job
tc run list --job MyProject_Build

# Only failures from the last 24 hours
tc run list --status failure --since 24h

# Builds on a specific branch
tc run list --branch main --limit 10
```

## Start a build

Trigger a new build by specifying a job ID:

```Shell
tc run start MyProject_Build
```

Add `--watch` to follow the build in real time:

```Shell
tc run start MyProject_Build --branch main --watch
```

The `--watch` flag displays a live progress view that updates until the build completes.

## View build logs

View the log output from a specific build:

```Shell
tc run log 12345
```

Or get the latest log for a job:

```Shell
tc run log --job MyProject_Build
```

The log output opens in a pager by default. Use `/` to search, `n`/`N` to navigate matches, and `q` to quit. Pass `--raw` to bypass the pager.

## Check the build queue

See what builds are waiting to run:

```Shell
tc queue list
```

## View build agents

List all registered build agents and their status:

```Shell
tc agent list
```

Filter to show only connected agents:

```Shell
tc agent list --connected
```

## Open a build in the browser

Most view commands support a `--web` flag that opens the corresponding page in your browser:

```Shell
tc run view 12345 --web
tc job view MyProject_Build --web
tc project view MyProject --web
```

## Enable shell completion

Set up tab completion for your shell:

```Shell
# Bash
tc completion bash > /etc/bash_completion.d/tc

# Zsh
tc completion zsh > "${fpath[1]}/_tc"

# Fish
tc completion fish > ~/.config/fish/completions/tc.fish

# PowerShell
tc completion powershell > tc.ps1
```

## Next steps

- Learn about [authentication methods](teamcity-cli-authentication.md) including multi-server setup and CI/CD usage.
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
