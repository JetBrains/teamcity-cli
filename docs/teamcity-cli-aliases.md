[//]: # (title: TeamCity CLI Aliases)

<show-structure for="chapter" depth="2"/>

Aliases let you create custom shortcuts for frequently used `tc` commands. They are stored in the [configuration file](teamcity-cli-configuration.md) and expand automatically when you run them.

## Creating aliases

Create an alias with `tc alias set`:

```Shell
tc alias set rl 'run list'
```

Now `tc rl` expands to `tc run list`.

### Positional arguments

Use `$1`, `$2`, and so on for positional arguments:

```Shell
tc alias set rw 'run view $1 --web'
```

Now `tc rw 12345` expands to `tc run view 12345 --web`.

Extra arguments that do not match a placeholder are appended to the end of the expanded command.

### Shell aliases

For aliases that need pipes, redirection, or other shell features, prefix the expansion with `!` or use the `--shell` flag:

```Shell
tc alias set watchnotify '!tc run watch $1 && notify-send "Build $1 done"'
tc alias set faillog '!tc run list --status=failure --json | jq ".[].id"'
```

Shell aliases are evaluated through `sh` instead of being expanded directly.

## Listing aliases

View all configured aliases:

```Shell
tc alias list
tc alias list --json
```

## Deleting aliases

Remove an alias:

```Shell
tc alias delete rl
```

## Useful alias examples

Here is a collection of commonly useful aliases:

### Quick shortcuts

```Shell
tc alias set rl       'run list'
tc alias set rv       'run view $1'
tc alias set rw       'run view $1 --web'
tc alias set jl       'job list'
tc alias set ql       'queue list'
```

### Filtered views

```Shell
tc alias set mine     'run list --user=@me'
tc alias set fails    'run list --status=failure --since=24h'
tc alias set running  'run list --status=running'
tc alias set morning  'run list --status=failure --since=12h'
```

### Build workflows

```Shell
tc alias set go       'run start $1 --watch'
tc alias set try      'run start $1 --local-changes --watch'
tc alias set hotfix   'run start $1 --top --clean --watch'
tc alias set retry    'run restart $1 --watch'
```

### Queue management

```Shell
tc alias set rush     'queue top $1'
tc alias set ok       'queue approve $1'
```

### Agent operations

```Shell
tc alias set maint    'agent disable $1'
tc alias set unmaint  'agent enable $1'
```

### API shortcuts

```Shell
tc alias set whoami   'api /app/rest/users/current'
```

### Shell aliases with external tools

```Shell
tc alias set watchnotify '!tc run watch $1 && notify-send "Build $1 done"'
tc alias set faillog     '!tc run list --status=failure --json | jq ".[].id"'
```

## Storage

Aliases are stored in the `aliases` section of `~/.config/tc/config.yml`:

```yaml
aliases:
  rl: 'run list'
  rw: 'run view $1 --web'
  mine: 'run list --user=@me'
  fails: 'run list --status=failure --since=24h'
```

You can also edit this file directly.

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
        <a href="teamcity-cli-configuration.md">Configuration</a>
    </category>
</seealso>
