[//]: # (title: TeamCity CLI Configuration)

<show-structure for="chapter" depth="2"/>

This page describes the configuration file format, environment variables, and shell completion setup for TeamCity CLI.

## Configuration file

TeamCity CLI stores its configuration in a YAML file at `~/.config/tc/config.yml`. This file is created automatically when you run `teamcity auth login`.

A typical configuration file looks like this:

```yaml
default_server: https://teamcity.example.com
servers:
  https://teamcity.example.com:
    user: alice
  https://teamcity-staging.example.com:
    user: alice
    guest: true
aliases:
  rl: 'run list'
  rw: 'run view $1 --web'
  mine: 'run list --user=@me'
```

### Configuration fields

<table>
<tr>
<td>

Field

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`default_server`

</td>
<td>

The server URL used when no `TEAMCITY_URL` environment variable is set. Updated automatically when you run `teamcity auth login`.

</td>
</tr>
<tr>
<td>

`servers`

</td>
<td>

A map of server URLs to their settings. Each entry stores the `user` field (username on that server) and optionally `guest: true` for guest access. Tokens are stored in the system keyring, not in this file, unless `--insecure-storage` was used during login.

</td>
</tr>
<tr>
<td>

`aliases`

</td>
<td>

A map of alias names to their expansions. See [Aliases](teamcity-cli-aliases.md) for details.

</td>
</tr>
</table>

## Environment variables

Environment variables override configuration file settings and are the recommended way to configure the CLI in CI/CD pipelines.

<table>
<tr>
<td>

Variable

</td>
<td>

Description

</td>
</tr>
<tr>
<td>

`TEAMCITY_URL`

</td>
<td>

TeamCity server URL. Takes precedence over `default_server` in the config file.

</td>
</tr>
<tr>
<td>

`TEAMCITY_TOKEN`

</td>
<td>

Access token for authentication. Takes precedence over the keyring and config file token.

</td>
</tr>
<tr>
<td>

`TEAMCITY_GUEST`

</td>
<td>

Set to `1` to use guest authentication (read-only, no token needed). Must be used with `TEAMCITY_URL`.

</td>
</tr>
<tr>
<td>

`TEAMCITY_DSL_DIR`

</td>
<td>

Path to the Kotlin DSL directory. Overrides automatic detection of `.teamcity/` directory.

</td>
</tr>
<tr>
<td>

`NO_COLOR`

</td>
<td>

Disable colored output. Follows the [NO_COLOR standard](https://no-color.org/).

</td>
</tr>
<tr>
<td>

`PAGER`

</td>
<td>

Pager program to use for commands that produce long output (for example, `teamcity run log`). Defaults to the system pager.

</td>
</tr>
</table>

Setting `TERM=dumb` also disables colored output. Color is automatically disabled when output is not a terminal (for example, when piping to another command).

## Global flags

These flags are available on every command:

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

`-h`, `--help`

</td>
<td>

Show help for the command.

</td>
</tr>
<tr>
<td>

`-v`, `--version`

</td>
<td>

Show the CLI version.

</td>
</tr>
<tr>
<td>

`--no-color`

</td>
<td>

Disable colored output.

</td>
</tr>
<tr>
<td>

`-q`, `--quiet`

</td>
<td>

Suppress non-essential output. Mutually exclusive with `--verbose`.

</td>
</tr>
<tr>
<td>

`--verbose`

</td>
<td>

Show detailed output, including debug information. Mutually exclusive with `--quiet`.

</td>
</tr>
<tr>
<td>

`--no-input`

</td>
<td>

Disable interactive prompts. The CLI uses sensible defaults when a prompt would otherwise appear.

</td>
</tr>
</table>

## Shell completion

TeamCity CLI supports tab completion for Bash, Zsh, Fish, and PowerShell. Completion covers commands, subcommands, flags, and in some cases values such as project and job IDs.

### Bash

```Shell
teamcity completion bash > /etc/bash_completion.d/teamcity
```

If you do not have write access to `/etc/bash_completion.d/`, write to a user-level location and source it from your `.bashrc`:

```Shell
teamcity completion bash > ~/.teamcity-completion.bash
echo 'source ~/.teamcity-completion.bash' >> ~/.bashrc
```

### Zsh

```Shell
teamcity completion zsh > "${fpath[1]}/_teamcity"
```

Ensure your `~/.zshrc` includes `compinit`:

```Shell
autoload -Uz compinit && compinit
```

### Fish

```Shell
teamcity completion fish > ~/.config/fish/completions/teamcity.fish
```

### PowerShell

```Shell
teamcity completion powershell > teamcity.ps1
. ./teamcity.ps1
```

To load completion automatically, add the output to your PowerShell profile.

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-authentication.md">Authentication</a>
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-aliases.md">Aliases</a>
    </category>
</seealso>
