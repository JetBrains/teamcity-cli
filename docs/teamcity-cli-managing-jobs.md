[//]: # (title: Managing Jobs)

<show-structure for="chapter" depth="2"/>

Jobs represent build configurations in TeamCity. The `tc job` command group lets you list and view build configurations, pause and resume them, and manage their parameters.

> In TeamCity CLI, "job" is equivalent to "build configuration" in the TeamCity web interface. See [TeamCity CLI](teamcity-cli.md#terminology-mapping) for the full terminology mapping.

## Listing jobs

View all build configurations:

```Shell
tc job list
```

Filter by project:

```Shell
tc job list --project MyProject
```

Limit the number of results:

```Shell
tc job list --limit 20
```

Output as JSON:

```Shell
tc job list --json
tc job list --json=id,name,projectName,webUrl
```

### job list flags

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

`-p`, `--project`

</td>
<td>

Filter by project ID

</td>
</tr>
<tr>
<td>

`-n`, `--limit`

</td>
<td>

Maximum number of jobs to display

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
</table>

## Viewing job details

View details of a build configuration:

```Shell
tc job view MyProject_Build
```

Open the job page in your browser:

```Shell
tc job view MyProject_Build --web
```

Output as JSON:

```Shell
tc job view MyProject_Build --json
```

## Pausing and resuming jobs

Pause a job to prevent new builds from being triggered:

```Shell
tc job pause MyProject_Build
```

Resume a paused job to allow new builds:

```Shell
tc job resume MyProject_Build
```

> Pausing a job stops all triggers from starting new builds. Builds that are already running or queued are not affected.
>
{style="note"}

## Managing job parameters

### Listing parameters

View all parameters defined on a job:

```Shell
tc job param list MyProject_Build
tc job param list MyProject_Build --json
```

### Getting a parameter value

Retrieve the value of a specific parameter:

```Shell
tc job param get MyProject_Build VERSION
tc job param get MyProject_Build env.JAVA_HOME
```

### Setting a parameter

Set or update a parameter value:

```Shell
tc job param set MyProject_Build VERSION "2.0.0"
```

To mark a parameter as secure (password field), use the `--secure` flag. Secure parameters have their values hidden in the web interface and build logs:

```Shell
tc job param set MyProject_Build SECRET_KEY "my-secret-value" --secure
```

### Deleting a parameter

Remove a parameter from a job:

```Shell
tc job param delete MyProject_Build MY_PARAM
```

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-managing-runs.md">Managing runs</a>
        <a href="teamcity-cli-managing-projects.md">Managing projects</a>
    </category>
</seealso>
