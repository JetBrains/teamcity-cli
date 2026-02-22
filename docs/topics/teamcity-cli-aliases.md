[//]: # (title: Aliases)

<show-structure for="chapter" depth="2"/>

Aliases let you create custom shortcuts for frequently used `teamcity` commands. They are stored in the [configuration file](teamcity-cli-configuration.md) and expand automatically when you run them.

## Creating aliases

Create an alias with `teamcity alias set`:

```Shell
teamcity alias set rl 'run list'
```

Now `teamcity rl` expands to `teamcity run list`.

<img src="alias-workflow.gif" alt="Creating and using aliases" border-effect="rounded"/>

### Positional arguments

Use `$1`, `$2`, and so on for positional arguments:

```Shell
teamcity alias set rw 'run view $1 --web'
```

Now `teamcity rw 12345` expands to `teamcity run view 12345 --web`.

Extra arguments that do not match a placeholder are appended to the end of the expanded command.

### Shell aliases

For aliases that need pipes, redirection, or other shell features, prefix the expansion with `!` or use the `--shell` flag:

```Shell
teamcity alias set watchnotify '!teamcity run watch $1 && notify-send "Build $1 done"'
teamcity alias set faillog '!teamcity run list --status=failure --json | jq ".[].id"'
```

Shell aliases are evaluated through `sh` instead of being expanded directly.

## Listing aliases

View all configured aliases:

```Shell
teamcity alias list
teamcity alias list --json
```

## Deleting aliases

Remove an alias:

```Shell
teamcity alias delete rl
```

## Useful alias examples

Here is a collection of commonly useful aliases:

### Quick shortcuts

```Shell
teamcity alias set rl       'run list'
teamcity alias set rv       'run view $1'
teamcity alias set rw       'run view $1 --web'
teamcity alias set jl       'job list'
teamcity alias set ql       'queue list'
```

### Filtered views

```Shell
teamcity alias set mine     'run list --user=@me'
teamcity alias set fails    'run list --status=failure --since=24h'
teamcity alias set running  'run list --status=running'
teamcity alias set morning  'run list --status=failure --since=12h'
```

### Build workflows

```Shell
teamcity alias set go       'run start $1 --watch'
teamcity alias set try      'run start $1 --local-changes --watch'
teamcity alias set hotfix   'run start $1 --top --clean --watch'
teamcity alias set retry    'run restart $1 --watch'
```

### Queue management

```Shell
teamcity alias set rush     'queue top $1'
teamcity alias set ok       'queue approve $1'
```

### Agent operations

```Shell
teamcity alias set maint    'agent disable $1'
teamcity alias set unmaint  'agent enable $1'
```

### API shortcuts

```Shell
teamcity alias set whoami   'api /app/rest/users/current'
```

### Shell aliases with external tools

```Shell
teamcity alias set watchnotify '!teamcity run watch $1 && notify-send "Build $1 done"'
teamcity alias set faillog     '!teamcity run list --status=failure --json | jq ".[].id"'
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
