[//]: # (title: Managing Projects)

<show-structure for="chapter" depth="2"/>

Projects organize build configurations and subprojects in TeamCity. The `teamcity project` command group lets you browse projects, manage parameters, handle secure tokens for versioned settings, and export or validate project configuration.

## Listing projects

View all TeamCity projects:

```Shell
teamcity project list
```

<img src="project-list.gif" alt="Listing TeamCity projects" border-effect="rounded"/>

Filter by parent project:

```Shell
teamcity project list --parent MyProject
```

Limit results and output as JSON:

```Shell
teamcity project list --limit 20
teamcity project list --json
teamcity project list --json=id,name,parentProjectId,webUrl
```

### project list flags

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

`-p`, `--parent`

</td>
<td>

Filter by parent project ID

</td>
</tr>
<tr>
<td>

`-n`, `--limit`

</td>
<td>

Maximum number of projects to display

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

## Project tree

Display the project hierarchy as a tree, including subprojects and build configurations:

```Shell
teamcity project tree
```

<img src="project-tree.gif" alt="Viewing project hierarchy tree" border-effect="rounded"/>

Show a specific subtree:

```Shell
teamcity project tree MyProject
```

Hide build configurations to see only the project structure:

```Shell
teamcity project tree --no-jobs
```

Limit the tree depth:

```Shell
teamcity project tree --depth 2
```

### project tree flags

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

`--no-jobs`

</td>
<td>

Hide build configurations, show only projects

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
</table>

## Viewing project details

View details of a project:

```Shell
teamcity project view MyProject
```

Open the project page in your browser:

```Shell
teamcity project view MyProject --web
```

Output as JSON:

```Shell
teamcity project view MyProject --json
```

## Managing project parameters

Project parameters are inherited by all build configurations within the project. They work identically to [job parameters](teamcity-cli-managing-jobs.md#managing-job-parameters).

### Listing parameters

```Shell
teamcity project param list MyProject
teamcity project param list MyProject --json
```

### Getting a parameter value

```Shell
teamcity project param get MyProject VERSION
```

### Setting a parameter

```Shell
teamcity project param set MyProject VERSION "2.0.0"
teamcity project param set MyProject SECRET_KEY "my-secret-value" --secure
```

### Deleting a parameter

```Shell
teamcity project param delete MyProject MY_PARAM
```

## Secure tokens

Secure tokens allow you to reference sensitive values (passwords, API keys) in versioned settings without storing them in version control. The actual values are kept securely in TeamCity and referenced using `credentialsJSON:<token>` identifiers.

### Storing a secure token

Store a sensitive value and receive a token reference:

```Shell
# Interactive prompt for the value
teamcity project token put MyProject

# Pass the value directly
teamcity project token put MyProject "my-secret-password"

# Read from stdin (useful for piping)
echo -n "my-secret" | teamcity project token put MyProject --stdin
```

The command returns a token in the format `credentialsJSON:<uuid>`. Use this token in your versioned settings configuration files.

> Storing a secure token requires the __Edit Project__ permission (Project Administrator role).
>
{style="note"}

### Retrieving a secure token value

Retrieve the original value for a secure token:

```Shell
teamcity project token get MyProject "credentialsJSON:abc123-def456..."
teamcity project token get MyProject "abc123-def456..."
```

> Retrieving secure token values requires the __Change Server Settings__ permission, which is only available to System Administrators.
>
{style="warning"}

## Versioned settings

### Exporting project settings

Export project settings as a ZIP archive containing Kotlin DSL or XML configuration:

```Shell
# Export as Kotlin DSL (default)
teamcity project settings export MyProject

# Export as Kotlin DSL explicitly
teamcity project settings export MyProject --kotlin

# Export as XML
teamcity project settings export MyProject --xml

# Save to a specific file
teamcity project settings export MyProject -o settings.zip

# Use relative IDs in the export
teamcity project settings export MyProject --relative-ids
```

The exported archive can be used to version control your CI/CD configuration, migrate settings between TeamCity instances, or review settings as code.

#### settings export flags

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

`--kotlin`

</td>
<td>

Export as Kotlin DSL (default)

</td>
</tr>
<tr>
<td>

`--xml`

</td>
<td>

Export as XML

</td>
</tr>
<tr>
<td>

`-o`, `--output`

</td>
<td>

Output file path (default: `projectSettings.zip`)

</td>
</tr>
<tr>
<td>

`--relative-ids`

</td>
<td>

Use relative IDs in the exported settings

</td>
</tr>
</table>

### Viewing versioned settings sync status

Check the synchronization status of versioned settings for a project:

```Shell
teamcity project settings status MyProject
teamcity project settings status MyProject --json
```

This displays whether versioned settings are enabled, the current sync state, last successful sync timestamp, VCS root and format information, and any errors from the last sync attempt.

### Validating Kotlin DSL

Validate Kotlin DSL configuration by running the TeamCity configuration generator:

```Shell
teamcity project settings validate
teamcity project settings validate ./path/to/.teamcity
teamcity project settings validate --verbose
```

The command auto-detects the `.teamcity` directory in the current directory or its parents. It requires Maven (`mvn`) or uses the Maven wrapper (`mvnw`) if present in the DSL directory.

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-managing-jobs.md">Managing jobs</a>
        <a href="teamcity-cli-managing-runs.md">Managing runs</a>
    </category>
</seealso>
