[//]: # (title: Managing the Build Queue)

<show-structure for="chapter" depth="2"/>

The build queue holds builds waiting to be assigned to an available agent. The `teamcity queue` command group lets you inspect queued builds, control their priority, approve builds that require manual approval, and remove builds from the queue.

## Listing queued builds

View all builds currently in the queue:

```Shell
teamcity queue list
```

Filter by job:

```Shell
teamcity queue list --job MyProject_Build
```

Limit results and output as JSON:

```Shell
teamcity queue list --limit 20
teamcity queue list --json
teamcity queue list --json=id,state,buildType.name,triggered.user.name,webUrl
```

### queue list flags

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

`-n`, `--limit`

</td>
<td>

Maximum number of queued runs to display

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

## Prioritizing a build

Move a queued build to the top of the queue, giving it the highest priority:

```Shell
teamcity queue top 12345
```

This is useful when a critical build needs to run before others in the queue.

## Approving a build

Some build configurations require manual approval before they can run. Approve a queued build:

```Shell
teamcity queue approve 12345
```

> Build approval is part of the TeamCity [deployment confirmation](configuring-build-triggers.md) workflow. Builds requiring approval remain in the queue until approved or removed.
>
{style="note"}

## Removing a build from the queue

Remove a build from the queue:

```Shell
teamcity queue remove 12345
```

Use `--force` to skip the confirmation prompt:

```Shell
teamcity queue remove 12345 --force
```

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-managing-runs.md">Managing runs</a>
        <a href="teamcity-cli-managing-agents.md">Managing agents</a>
    </category>
</seealso>
