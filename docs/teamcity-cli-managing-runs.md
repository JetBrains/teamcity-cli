[//]: # (title: Managing Runs)

<show-structure for="chapter" depth="2"/>

Runs represent build executions in TeamCity. The `teamcity run` command group lets you start builds, monitor them in real time, view logs and test results, manage artifacts, and organize runs with tags, comments, and pins.

> In TeamCity CLI, "run" is equivalent to "build" in the TeamCity web interface. See [TeamCity CLI](teamcity-cli.md#terminology-mapping) for the full terminology mapping.

## Listing runs

View recent builds with `teamcity run list`:

```Shell
teamcity run list
```

### Filtering

Use flags to narrow results:

```Shell
# Builds for a specific job
teamcity run list --job MyProject_Build

# Filter by project
teamcity run list --project MyProject

# Filter by branch
teamcity run list --branch main

# Filter by status
teamcity run list --status failure

# Filter by user who triggered the build
teamcity run list --user alice
teamcity run list --user @me

# Combine filters
teamcity run list --job MyProject_Build --status failure --branch main
```

> The `@me` shortcut substitutes the currently authenticated username.

### Time-based filtering

Use `--since` and `--until` to filter by time:

```Shell
# Builds from the last 24 hours
teamcity run list --since 24h

# Builds from a specific date onward
teamcity run list --since 2026-01-15

# Builds in a time range
teamcity run list --since 2026-01-15 --until 2026-01-20
```

### Limiting results

```Shell
teamcity run list --limit 20
```

### Output options

```Shell
# JSON output (see Scripting and automation for details)
teamcity run list --json
teamcity run list --json=id,status,webUrl

# Plain text for scripting
teamcity run list --plain
teamcity run list --plain --no-header
```

### run list flags

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

`-j`, `--job`

</td>
<td>

Filter by job (build configuration) ID

</td>
</tr>
<tr>
<td>

`-p`, `--project`

</td>
<td>

Filter by project ID

</td>
</tr>
<tr>
<td>

`-b`, `--branch`

</td>
<td>

Filter by branch name

</td>
</tr>
<tr>
<td>

`--status`

</td>
<td>

Filter by status: `success`, `failure`, `running`, `error`, or `unknown`

</td>
</tr>
<tr>
<td>

`-u`, `--user`

</td>
<td>

Filter by the user who triggered the build. Use `@me` for the current user.

</td>
</tr>
<tr>
<td>

`--since`

</td>
<td>

Show builds finished after this time (for example, `24h`, `2026-01-21`)

</td>
</tr>
<tr>
<td>

`--until`

</td>
<td>

Show builds finished before this time

</td>
</tr>
<tr>
<td>

`-n`, `--limit`

</td>
<td>

Maximum number of runs to display

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
<tr>
<td>

`--plain`

</td>
<td>

Tab-separated output for scripting

</td>
</tr>
<tr>
<td>

`--no-header`

</td>
<td>

Omit header row (use with `--plain`)

</td>
</tr>
<tr>
<td>

`-w`, `--web`

</td>
<td>

Open the list in the browser

</td>
</tr>
</table>

## Starting a run

Trigger a new build with `teamcity run start`:

```Shell
teamcity run start MyProject_Build
```

### Specifying a branch and revision

```Shell
# Build a specific branch
teamcity run start MyProject_Build --branch feature/login

# Pin to a specific Git commit
teamcity run start MyProject_Build --branch main --revision abc123def
```

### Build parameters

Pass custom parameters, system properties, and environment variables:

```Shell
teamcity run start MyProject_Build \
  -P version=1.0 \
  -S build.number=123 \
  -E CI=true
```

### Build options

```Shell
# Clean all source files before building
teamcity run start MyProject_Build --clean

# Rebuild all dependencies
teamcity run start MyProject_Build --rebuild-deps

# Rebuild only failed dependencies
teamcity run start MyProject_Build --rebuild-failed-deps

# Add to the top of the queue
teamcity run start MyProject_Build --top

# Run on a specific agent
teamcity run start MyProject_Build --agent 5
```

### Tags and comments

```Shell
teamcity run start MyProject_Build --tag release --tag v2.0 --comment "Release build"
```

### Start and watch

Add `--watch` to follow the build after starting it:

```Shell
teamcity run start MyProject_Build --branch main --watch
```

### Personal builds

Include uncommitted local changes in a personal build:

```Shell
# Auto-detect changes from Git working directory
teamcity run start MyProject_Build --local-changes

# From a patch file
teamcity run start MyProject_Build --local-changes changes.patch

# From stdin
git diff | teamcity run start MyProject_Build --local-changes -
```

By default, the CLI pushes your branch to the remote before starting a personal build. Use `--no-push` to skip this:

```Shell
teamcity run start MyProject_Build --local-changes --no-push
```

### Dry run

Preview what would be triggered without actually starting a build:

```Shell
teamcity run start MyProject_Build --dry-run
```

### run start flags

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

`-b`, `--branch`

</td>
<td>

Branch to build

</td>
</tr>
<tr>
<td>

`--revision`

</td>
<td>

Pin build to a specific Git commit SHA

</td>
</tr>
<tr>
<td>

`-P`, `--param`

</td>
<td>

Build parameters as `key=value` (can be repeated)

</td>
</tr>
<tr>
<td>

`-S`, `--system`

</td>
<td>

System properties as `key=value` (can be repeated)

</td>
</tr>
<tr>
<td>

`-E`, `--env`

</td>
<td>

Environment variables as `key=value` (can be repeated)

</td>
</tr>
<tr>
<td>

`-m`, `--comment`

</td>
<td>

Build comment

</td>
</tr>
<tr>
<td>

`-t`, `--tag`

</td>
<td>

Build tag (can be repeated)

</td>
</tr>
<tr>
<td>

`--personal`

</td>
<td>

Run as a personal build

</td>
</tr>
<tr>
<td>

`-l`, `--local-changes`

</td>
<td>

Include local changes. Accepts `git` (default), `-` (stdin), or a file path.

</td>
</tr>
<tr>
<td>

`--no-push`

</td>
<td>

Skip auto-push of branch to remote

</td>
</tr>
<tr>
<td>

`--clean`

</td>
<td>

Clean source files before building

</td>
</tr>
<tr>
<td>

`--rebuild-deps`

</td>
<td>

Rebuild all dependencies

</td>
</tr>
<tr>
<td>

`--rebuild-failed-deps`

</td>
<td>

Rebuild failed or incomplete dependencies only

</td>
</tr>
<tr>
<td>

`--top`

</td>
<td>

Add to the top of the build queue

</td>
</tr>
<tr>
<td>

`--agent`

</td>
<td>

Run on a specific agent (by ID)

</td>
</tr>
<tr>
<td>

`--watch`

</td>
<td>

Watch the build after starting it

</td>
</tr>
<tr>
<td>

`-n`, `--dry-run`

</td>
<td>

Preview without starting

</td>
</tr>
<tr>
<td>

`--json`

</td>
<td>

Output as JSON

</td>
</tr>
<tr>
<td>

`-w`, `--web`

</td>
<td>

Open run in browser

</td>
</tr>
</table>

## Viewing run details

```Shell
teamcity run view 12345
teamcity run view 12345 --web
teamcity run view 12345 --json
```

## Watching a run

Monitor a running build with live updates:

```Shell
teamcity run watch 12345
```

Stream build logs while watching:

```Shell
teamcity run watch 12345 --logs
```

Set a custom refresh interval or timeout:

```Shell
teamcity run watch 12345 --interval 10
teamcity run watch 12345 --timeout 30m
```

Use `--quiet` for minimal output that shows only state changes and the final result:

```Shell
teamcity run watch 12345 --quiet
```

### run watch flags

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

`-i`, `--interval`

</td>
<td>

Refresh interval in seconds

</td>
</tr>
<tr>
<td>

`--logs`

</td>
<td>

Stream build logs while watching

</td>
</tr>
<tr>
<td>

`-Q`, `--quiet`

</td>
<td>

Minimal output — only state changes and result

</td>
</tr>
<tr>
<td>

`--timeout`

</td>
<td>

Stop watching after this duration (for example, `30m`, `1h`)

</td>
</tr>
</table>

## Viewing build logs

View the log output from a run:

```Shell
teamcity run log 12345
```

View the log for the latest run of a specific job:

```Shell
teamcity run log --job MyProject_Build
```

Show only the failure summary (problems and failed tests):

```Shell
teamcity run log 12345 --failed
```

Bypass the pager and output raw text:

```Shell
teamcity run log 12345 --raw
```

> The log viewer uses a pager by default. Use `/` to search, `n`/`N` to navigate matches, `g`/`G` to jump to the top or bottom, and `q` to quit.
>
{style="tip"}

## Canceling a run

Cancel a running or queued build:

```Shell
teamcity run cancel 12345
teamcity run cancel 12345 --comment "Canceling for hotfix"
```

Use `--force` to skip the confirmation prompt:

```Shell
teamcity run cancel 12345 --force
```

## Restarting a run

Restart a run with the same configuration:

```Shell
teamcity run restart 12345
teamcity run restart 12345 --watch
teamcity run restart 12345 --web
```

## Artifacts

### Listing artifacts

List artifacts from a run without downloading them:

```Shell
teamcity run artifacts 12345
teamcity run artifacts --job MyProject_Build
teamcity run artifacts 12345 --path html_reports/coverage
teamcity run artifacts 12345 --json
```

### Downloading artifacts

Download artifacts from a completed run:

```Shell
teamcity run download 12345
teamcity run download 12345 --dir ./artifacts
teamcity run download 12345 --artifact "*.jar"
```

## Test results

Show test results from a run:

```Shell
teamcity run tests 12345
teamcity run tests --job MyProject_Build
```

Show only failed tests:

```Shell
teamcity run tests 12345 --failed
```

Limit the number of results:

```Shell
teamcity run tests 12345 --limit 50
teamcity run tests 12345 --json
```

## VCS changes

Show the VCS commits included in a run:

```Shell
teamcity run changes 12345
```

Show commits only (without file listings):

```Shell
teamcity run changes 12345 --no-files
teamcity run changes 12345 --json
```

## Pinning runs

Pin a run to prevent it from being cleaned up by retention policies:

```Shell
teamcity run pin 12345
teamcity run pin 12345 --comment "Release candidate"
```

Remove the pin:

```Shell
teamcity run unpin 12345
```

## Tagging runs

Add tags to a run for categorization and filtering:

```Shell
teamcity run tag 12345 release
teamcity run tag 12345 release v2.0 production
```

Remove tags:

```Shell
teamcity run untag 12345 release
teamcity run untag 12345 release v2.0
```

## Comments

Set a comment on a run:

```Shell
teamcity run comment 12345 "Deployed to production"
```

View the current comment:

```Shell
teamcity run comment 12345
```

Delete the comment:

```Shell
teamcity run comment 12345 --delete
```

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-managing-build-queue.md">Managing the build queue</a>
        <a href="teamcity-cli-managing-jobs.md">Managing jobs</a>
        <a href="teamcity-cli-scripting.md">Scripting and automation</a>
    </category>
</seealso>
