[//]: # (title: Managing Projects)

<show-structure for="chapter" depth="2"/>

Projects organize build configurations and subprojects in TeamCity. The `teamcity project` command group lets you browse projects, manage VCS roots, manage parameters, handle secure tokens for versioned settings, and export or validate project configuration.

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

## Managing VCS roots

VCS roots define the connection between TeamCity and your version control repository. They are project-level entities, visible to child projects through inheritance.

### Listing VCS roots

```Shell
teamcity project vcs list --project MyProject
teamcity project vcs list --project MyProject --json
teamcity project vcs list --project MyProject --plain
```

### Viewing VCS root details

```Shell
teamcity project vcs view MyProject_GitHubRepo
teamcity project vcs view MyProject_GitHubRepo --json
teamcity project vcs view MyProject_GitHubRepo --web
```

The view command shows all VCS root properties with human-readable labels. Secure properties (passwords, passphrases) are masked as `********`.

<img src="project-vcs.gif" alt="Listing and viewing VCS roots" border-effect="rounded"/>

### Creating a VCS root

Create a Git VCS root with an interactive wizard or flags:

```Shell
teamcity project vcs create --project MyProject
```

The wizard prompts for the repository URL, display name, and authentication method. Six auth methods are supported:

```Shell
# Anonymous (public repos)
teamcity project vcs create --url https://github.com/org/repo.git --auth anonymous

# Password / Personal Access Token
teamcity project vcs create --url https://github.com/org/repo.git \
  --auth password --username oauth2 --password ghp_xxx

# SSH key uploaded to TeamCity
teamcity project vcs create --url git@github.com:org/repo.git \
  --auth ssh-key --ssh-key-name my-deploy-key

# SSH key from the build agent's default key
teamcity project vcs create --url git@github.com:org/repo.git --auth ssh-agent

# SSH key at a custom path on the build agent
teamcity project vcs create --url git@github.com:org/repo.git \
  --auth ssh-file --key-path /path/to/key

# Access token via a project connection
teamcity project vcs create --url https://github.com/org/repo.git \
  --auth token --connection-id PROJECT_EXT_1
```

By default, the command tests the connection before creating. Skip with `--no-test`:

```Shell
teamcity project vcs create --url https://github.com/org/repo.git --auth anonymous --no-test
```

#### vcs create flags

<table>
<tr><td>Flag</td><td>Description</td></tr>
<tr><td><code>--url</code></td><td>Repository URL</td></tr>
<tr><td><code>--name</code></td><td>Display name (auto-generated from URL if omitted)</td></tr>
<tr><td><code>-p</code>, <code>--project</code></td><td>Project ID (default: _Root)</td></tr>
<tr><td><code>--auth</code></td><td>Auth method: <code>password</code>, <code>ssh-key</code>, <code>ssh-agent</code>, <code>ssh-file</code>, <code>token</code>, <code>anonymous</code></td></tr>
<tr><td><code>--username</code></td><td>Username (for password auth)</td></tr>
<tr><td><code>--password</code></td><td>Password or personal access token</td></tr>
<tr><td><code>--stdin</code></td><td>Read password from stdin</td></tr>
<tr><td><code>--ssh-key-name</code></td><td>Name of SSH key uploaded to TeamCity</td></tr>
<tr><td><code>--key-path</code></td><td>Path to SSH key file on the build agent</td></tr>
<tr><td><code>--passphrase</code></td><td>SSH key passphrase</td></tr>
<tr><td><code>--connection-id</code></td><td>OAuth connection ID</td></tr>
<tr><td><code>--branch</code></td><td>Default branch (default: <code>refs/heads/main</code>)</td></tr>
<tr><td><code>--branch-spec</code></td><td>Branch specification</td></tr>
<tr><td><code>--no-test</code></td><td>Skip connection test before creating</td></tr>
</table>

### Testing a VCS root connection

Test whether an existing VCS root can connect to the repository:

```Shell
teamcity project vcs test MyProject_GitHubRepo
```

### Deleting a VCS root

```Shell
teamcity project vcs delete MyProject_GitHubRepo
teamcity project vcs delete MyProject_GitHubRepo --force  # skip confirmation
```

## Managing SSH keys

SSH keys uploaded to a project can be used for VCS root authentication (`TEAMCITY_SSH_KEY` auth method). Keys are inherited by child projects.

### Listing SSH keys

```Shell
teamcity project ssh list --project MyProject
teamcity project ssh list --project MyProject --json
```

### Generating an SSH key pair

Generate an ed25519 or RSA key pair directly in TeamCity and print the public key:

```Shell
teamcity project ssh generate --name deploy-key --project MyProject
teamcity project ssh generate --name deploy-key --type rsa --project MyProject
```

Add the printed public key as a deploy key in your Git hosting provider.

### Uploading an SSH key

```Shell
teamcity project ssh upload ~/.ssh/id_ed25519 --project MyProject
teamcity project ssh upload key.pem --name my-deploy-key --project MyProject
```

### Deleting an SSH key

```Shell
teamcity project ssh delete my-deploy-key --project MyProject
teamcity project ssh delete my-deploy-key --project MyProject --force
```

## Project connections

Connections (OAuth providers, Docker registries, cloud integrations) are configured at the project level. The CLI can list them for use with `vcs create --auth token`.

### Listing connections

```Shell
teamcity project connection list --project MyProject
teamcity project connection list --project MyProject --json
```

## Managing project parameters

Project parameters are inherited by all build configurations within the project. They work identically to [job parameters](teamcity-cli-managing-jobs.md#Managing+job+parameters).

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
