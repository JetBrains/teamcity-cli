[//]: # (title: Managing Jobs)

<show-structure for="chapter" depth="2"/>

Jobs represent build configurations in TeamCity. The `teamcity job` command group lets you list and view build configurations, pause and resume them, and manage their parameters.

> In TeamCity CLI, "job" is equivalent to "build configuration" in the TeamCity web interface. See the [Glossary](teamcity-cli-glossary.md) for the full terminology mapping.

## Listing jobs

View all build configurations:

```Shell
teamcity job list
```

<img src="job-list.gif" alt="Listing and filtering jobs" border-effect="rounded"/>

Filter by project:

```Shell
teamcity job list --project MyProject
```

Limit the number of results (use `--limit 0` to fetch all):

```Shell
teamcity job list --limit 20
teamcity job list --limit 0
```

Output as JSON:

```Shell
teamcity job list --json
teamcity job list --json=id,name,projectName,webUrl
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

Maximum number of jobs to display (use 0 for all)

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
teamcity job view MyProject_Build
```

Open the job page in your browser:

```Shell
teamcity job view MyProject_Build --web
```

Output as JSON:

```Shell
teamcity job view MyProject_Build --json
```

## Dependency tree

Visualize the snapshot dependency chain for a job. By default, the tree shows both dependents (what gets triggered after this job) and dependencies (what must run before this job):

```Shell
teamcity job tree MyProject_DeployStaging
```

<img src="job-tree.gif" alt="Viewing job dependency trees" border-effect="rounded"/>

Show only one direction:

```Shell
teamcity job tree MyProject_Build --only dependents
teamcity job tree MyProject_Deploy --only dependencies
```

Limit the tree depth:

```Shell
teamcity job tree MyProject_Build --depth 2
```

### job tree flags

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

`-d`, `--depth`

</td>
<td>

Limit tree depth (0 = unlimited)

</td>
</tr>
<tr>
<td>

`--only`

</td>
<td>

Show only `dependents` or `dependencies`

</td>
</tr>
</table>

## Pausing and resuming jobs

Pause a job to prevent new builds from being triggered:

```Shell
teamcity job pause MyProject_Build
```

Resume a paused job to allow new builds:

```Shell
teamcity job resume MyProject_Build
```

> Pausing a job stops all triggers from starting new builds. Builds that are already running or queued are not affected.
>
{style="note"}

## Managing job parameters

### Listing parameters

View all parameters defined on a job:

```Shell
teamcity job param list MyProject_Build
teamcity job param list MyProject_Build --json
```

### Getting a parameter value

Retrieve the value of a specific parameter:

```Shell
teamcity job param get MyProject_Build VERSION
teamcity job param get MyProject_Build env.JAVA_HOME
```

### Setting a parameter

Set or update a parameter value:

```Shell
teamcity job param set MyProject_Build VERSION "2.0.0"
```

To mark a parameter as secure (password field), use the `--secure` flag. Secure parameters have their values hidden in the web interface and build logs:

```Shell
teamcity job param set MyProject_Build SECRET_KEY "my-secret-value" --secure
```

### Deleting a parameter

Remove a parameter from a job:

```Shell
teamcity job param delete MyProject_Build MY_PARAM
```

## Managing job settings

Settings are the build-configuration options that control how a job runs — build
number format, execution timeout, artifact rules, and similar. Unlike
parameters, settings always have server defaults and cannot be deleted.

### Listing settings

View all settings defined on a job:

```Shell
teamcity job settings list MyProject_Build
teamcity job settings list MyProject_Build --json
```

### Getting a setting value

Retrieve the value of a specific setting:

```Shell
teamcity job settings get MyProject_Build buildNumberPattern
teamcity job settings get MyProject_Build executionTimeoutMin
```

### Setting a value

Set or update a setting value:

```Shell
teamcity job settings set MyProject_Build buildNumberPattern "2.0.%build.counter%"
teamcity job settings set MyProject_Build executionTimeoutMin 30
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
