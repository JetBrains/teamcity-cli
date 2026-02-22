[//]: # (title: Scripting and Automation)

<show-structure for="chapter" depth="2"/>

TeamCity CLI provides several output formats and features designed for scripting, automation, and CI/CD integration.

## JSON output

Many commands support a `--json` flag for machine-readable output. List commands also accept optional field selection.

### Basic usage

```Shell
teamcity run list --json
teamcity job list --json
teamcity project list --json
```

### Discovering available fields

Pass `--json=` (with an empty value) to see all available fields for a command:

```Shell
teamcity run list --json=
```

<img src="json-output.gif" alt="JSON output with field selection" border-effect="rounded"/>

### Selecting specific fields

Specify a comma-separated list of fields:

```Shell
teamcity run list --json=id,status,webUrl
```

Field selection (`--json=...`) is available on list commands only. View and inspection commands accept `--json` without field selection.

### Nested fields

Use dot notation to access nested fields:

```Shell
teamcity run list --json=id,status,buildType.name,triggered.user.username
```

### JSON for view and inspection commands

```Shell
teamcity run view 12345 --json
teamcity run changes 12345 --json
teamcity run tests 12345 --json
teamcity run artifacts 12345 --json
teamcity agent view Agent-Linux-01 --json
teamcity project settings status MyProject --json
```

### Available fields by command

<table>
<tr>
<td>

Command

</td>
<td>

Example fields

</td>
</tr>
<tr>
<td>

`teamcity run list`

</td>
<td>

`id`, `number`, `status`, `state`, `branchName`, `buildTypeId`, `buildType.name`, `buildType.projectName`, `triggered.type`, `triggered.user.name`, `agent.name`, `startDate`, `finishDate`, `webUrl`

</td>
</tr>
<tr>
<td>

`teamcity job list`

</td>
<td>

`id`, `name`, `projectName`, `projectId`, `paused`, `href`, `webUrl`

</td>
</tr>
<tr>
<td>

`teamcity project list`

</td>
<td>

`id`, `name`, `description`, `parentProjectId`, `href`, `webUrl`

</td>
</tr>
<tr>
<td>

`teamcity queue list`

</td>
<td>

`id`, `buildTypeId`, `state`, `branchName`, `queuedDate`, `buildType.name`, `triggered.user.name`, `webUrl`

</td>
</tr>
<tr>
<td>

`teamcity agent list`

</td>
<td>

`id`, `name`, `connected`, `enabled`, `authorized`, `pool.name`, `webUrl`

</td>
</tr>
<tr>
<td>

`teamcity pool list`

</td>
<td>

`id`, `name`, `maxAgents`

</td>
</tr>
</table>

## Plain text output

Use `--plain` for tab-separated output that is easy to parse with standard Unix tools:

```Shell
teamcity run list --plain
```

Omit the header row for cleaner piping:

```Shell
teamcity run list --plain --no-header
```

## Scripting examples

### Get IDs of failed builds

```Shell
teamcity run list --status failure --json=id | jq -r '.[].id'
```

### Export build data to CSV

```Shell
teamcity run list --json=id,status,branchName | jq -r '.[] | [.id,.status,.branchName] | @csv'
```

### Get web URLs for queued builds

```Shell
teamcity queue list --json=webUrl | jq -r '.[].webUrl'
```

### Count builds by status

```Shell
teamcity run list --since 24h --json=status | jq 'group_by(.status) | map({status: .[0].status, count: length})'
```

### Wait for a build to finish

```Shell
teamcity run start MyProject_Build --json | jq -r '.id' | xargs teamcity run watch --quiet
```

### Cancel all queued builds for a job

```Shell
teamcity queue list --job MyProject_Build --json=id | jq -r '.[].id' | xargs -I {} teamcity run cancel {} --force
```

## CI/CD integration

### Environment variable authentication

In CI/CD pipelines, use environment variables for authentication:

<tabs>
<tab title="macOS and Linux">

```Shell
export TEAMCITY_URL="https://teamcity.example.com"
export TEAMCITY_TOKEN="your-access-token"
```

</tab>
<tab title="Windows">

PowerShell:

```PowerShell
$env:TEAMCITY_URL = "https://teamcity.example.com"
$env:TEAMCITY_TOKEN = "your-access-token"
```

CMD:

```Shell
set TEAMCITY_URL=https://teamcity.example.com
set TEAMCITY_TOKEN=your-access-token
```

</tab>
</tabs>

See [Authentication](teamcity-cli-authentication.md#environment-variables) for details.

### Non-interactive mode

Use `--no-input` to disable interactive prompts in automated environments. The CLI uses sensible defaults when prompts are suppressed:

```Shell
teamcity run cancel 12345 --no-input
```

Alternatively, use `--force` on commands that support it:

```Shell
teamcity queue remove 12345 --force
```

### Quiet mode

Use `--quiet` to suppress non-essential output:

```Shell
teamcity run start MyProject_Build --quiet
```

### Exit codes

Most commands return exit code `0` on success and `1` on failure. The `teamcity run watch` flow (including `teamcity run start --watch`) returns:

- `2` when a run is canceled
- `124` on timeout

```Shell
teamcity run start MyProject_Build --watch --quiet
case $? in
  0) echo "Build succeeded" ;;
  1) echo "Build failed" ;;
  2) echo "Build cancelled" ;;
  124) echo "Timed out" ;;
  *) echo "Unknown error" ;;
esac
```

## Raw API access

For operations not covered by dedicated commands, use `teamcity api` to make direct REST API requests:

```Shell
teamcity api /app/rest/server
teamcity api /app/rest/builds --paginate --slurp
```

See [REST API access](teamcity-cli-rest-api-access.md) for details.

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
        <a href="teamcity-cli-rest-api-access.md">REST API access</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-aliases.md">Aliases</a>
        <a href="teamcity-cli-authentication.md">Authentication</a>
    </category>
</seealso>
