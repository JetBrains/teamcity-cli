[//]: # (title: Scripting and Automation with TeamCity CLI)

<show-structure for="chapter" depth="2"/>

TeamCity CLI provides several output formats and features designed for scripting, automation, and CI/CD integration.

## JSON output

Commands that list or view resources support a `--json` flag for machine-readable output.

### Basic usage

```Shell
tc run list --json
tc job list --json
tc project list --json
```

### Discovering available fields

Pass `--json=` (with an empty value) to see all available fields for a command:

```Shell
tc run list --json=
```

### Selecting specific fields

Specify a comma-separated list of fields:

```Shell
tc run list --json=id,status,webUrl
```

### Nested fields

Use dot notation to access nested fields:

```Shell
tc run list --json=id,status,buildType.name,triggered.user.username
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

`tc run list`

</td>
<td>

`id`, `number`, `status`, `state`, `branchName`, `buildTypeId`, `buildType.name`, `buildType.projectName`, `triggered.type`, `triggered.user.name`, `agent.name`, `startDate`, `finishDate`, `webUrl`

</td>
</tr>
<tr>
<td>

`tc job list`

</td>
<td>

`id`, `name`, `projectName`, `projectId`, `paused`, `href`, `webUrl`

</td>
</tr>
<tr>
<td>

`tc project list`

</td>
<td>

`id`, `name`, `description`, `parentProjectId`, `href`, `webUrl`

</td>
</tr>
<tr>
<td>

`tc queue list`

</td>
<td>

`id`, `buildTypeId`, `state`, `branchName`, `queuedDate`, `buildType.name`, `triggered.user.name`, `webUrl`

</td>
</tr>
<tr>
<td>

`tc agent list`

</td>
<td>

`id`, `name`, `connected`, `enabled`, `authorized`, `pool.name`, `webUrl`

</td>
</tr>
<tr>
<td>

`tc pool list`

</td>
<td>

`id`, `name`, `maxAgents`

</td>
</tr>
</table>

## Plain text output

Use `--plain` for tab-separated output that is easy to parse with standard Unix tools:

```Shell
tc run list --plain
```

Omit the header row for cleaner piping:

```Shell
tc run list --plain --no-header
```

## Scripting examples

### Get IDs of failed builds

```Shell
tc run list --status failure --json=id | jq -r '.[].id'
```

### Export build data to CSV

```Shell
tc run list --json=id,status,branchName | jq -r '.[] | [.id,.status,.branchName] | @csv'
```

### Get web URLs for queued builds

```Shell
tc queue list --json=webUrl | jq -r '.[].webUrl'
```

### Count builds by status

```Shell
tc run list --since 24h --json=status | jq 'group_by(.status) | map({status: .[0].status, count: length})'
```

### Wait for a build to finish

```Shell
tc run start MyProject_Build --json | jq -r '.id' | xargs tc run watch --quiet
```

### Cancel all queued builds for a job

```Shell
tc queue list --job MyProject_Build --json=id | jq -r '.[].id' | xargs -I {} tc run cancel {} --force
```

## CI/CD integration

### Environment variable authentication

In CI/CD pipelines, use environment variables for authentication:

```Shell
export TEAMCITY_URL="https://teamcity.example.com"
export TEAMCITY_TOKEN="your-access-token"
```

See [Authentication](teamcity-cli-authentication.md#environment-variables) for details.

### Non-interactive mode

Use `--no-input` to disable interactive prompts in automated environments. The CLI uses sensible defaults when prompts are suppressed:

```Shell
tc run cancel 12345 --no-input
```

Alternatively, use `--force` on commands that support it:

```Shell
tc queue remove 12345 --force
```

### Quiet mode

Use `--quiet` to suppress non-essential output:

```Shell
tc run start MyProject_Build --quiet
```

### Exit codes

The CLI returns exit code `0` on success and `1` on failure. Use this in scripts to detect errors:

```Shell
if tc run start MyProject_Build --watch --quiet; then
  echo "Build succeeded"
else
  echo "Build failed"
  exit 1
fi
```

### GitHub Actions example

```yaml
jobs:
  trigger-build:
    runs-on: ubuntu-latest
    steps:
      - name: Install TeamCity CLI
        run: curl -fsSL https://jb.gg/tc/install | bash

      - name: Trigger build
        env:
          TEAMCITY_URL: ${{ secrets.TEAMCITY_URL }}
          TEAMCITY_TOKEN: ${{ secrets.TEAMCITY_TOKEN }}
        run: |
          tc run start MyProject_Build \
            --branch "${{ github.ref_name }}" \
            --watch --quiet
```

## Raw API access

For operations not covered by dedicated commands, use `tc api` to make direct REST API requests:

```Shell
tc api /app/rest/server
tc api /app/rest/builds --paginate --slurp
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
