[//]: # (title: Managing Pipelines)

<show-structure for="chapter" depth="2"/>

Pipelines are a YAML-based configuration system for defining CI/CD workflows in TeamCity. Each pipeline belongs to a project, references a VCS root, and contains one or more jobs. The `teamcity pipeline` command group lets you list, inspect, create, and manage pipeline YAML.

> For background on how pipelines work in TeamCity, see [Create and Edit Pipelines](https://www.jetbrains.com/help/teamcity/create-and-edit-pipelines.html).
>
{style="note"}

## Listing pipelines

View all pipelines on the server:

```Shell
teamcity pipeline list
```

<img src="pipeline-list.gif" alt="Listing pipelines" border-effect="rounded"/>

Filter by project:

```Shell
teamcity pipeline list --project MyProject
teamcity pipeline list --project MyProject --json
```

### pipeline list flags

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

Maximum number of pipelines to display (default 30)

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

## Viewing pipeline details

```Shell
teamcity pipeline view CLI_CiCd
teamcity pipeline view CLI_CiCd --json
```

The view shows the pipeline name, parent project, head build type, and all jobs with their YAML keys and display names.

## Validating pipeline YAML

Validate a `.teamcity.yml` file against the server's JSON schema before pushing:

```Shell
teamcity pipeline validate
teamcity pipeline validate path/to/pipeline.yml
```

The schema is fetched from the server and cached locally for 24 hours. Use `--refresh-schema` to force a re-fetch, or `--schema` to use a local schema file.

### validate flags

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

`--schema`

</td>
<td>

Path to a local JSON schema file

</td>
</tr>
<tr>
<td>

`--refresh-schema`

</td>
<td>

Force re-fetch schema from server

</td>
</tr>
</table>

## Creating a pipeline

Create a new pipeline from a YAML file:

```Shell
teamcity pipeline create my-pipeline --project CLI
teamcity pipeline create my-pipeline --project CLI --vcs-root MyVcsRoot
teamcity pipeline create my-pipeline --project CLI --file pipeline.yml
```

When `--vcs-root` is omitted, an interactive prompt lists VCS roots available in the project.

### create flags

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

Parent project ID (required)

</td>
</tr>
<tr>
<td>

`--vcs-root`

</td>
<td>

VCS root ID. Use `teamcity project vcs list` to discover available roots.

</td>
</tr>
<tr>
<td>

`-f`, `--file`

</td>
<td>

Path to pipeline YAML file (default `.teamcity.yml`)

</td>
</tr>
</table>

## Downloading pipeline YAML

Download the YAML source for a server-stored pipeline:

```Shell
teamcity pipeline pull CLI_CiCd
teamcity pipeline pull CLI_CiCd -o .teamcity.yml
```

Output goes to stdout by default, making it pipe-friendly. Use `-o` to write to a file.

> Pipelines that store YAML in the VCS repository cannot be pulled. The CLI detects this and suggests editing the file in your repo directly.
>
{style="note"}

## Uploading pipeline YAML

Push updated YAML to a server-stored pipeline:

```Shell
teamcity pipeline push CLI_CiCd
teamcity pipeline push CLI_CiCd pipeline.yml
```

When no file is specified, the CLI reads `.teamcity.yml` from the current directory.

## Deleting a pipeline

```Shell
teamcity pipeline delete CLI_MyPipeline
teamcity pipeline delete CLI_MyPipeline --force
```

A confirmation prompt is shown unless `--force` is passed.

## Managing secrets

Pipeline secrets use `credentialsJSON:` tokens. Create a token in the pipeline's project, then reference it in your YAML:

```Shell
# Create the pipeline first
teamcity pipeline create my-pipeline --project CLI

# Store a secret in the pipeline's project
teamcity project token put CLI_MyPipeline "my-api-key"
# Output: credentialsJSON:2d3c2507-4840-...
```

Reference the token in your pipeline YAML:

```yaml
secrets:
  env.API_KEY: credentialsJSON:2d3c2507-4840-...
```

Then push:

```Shell
teamcity pipeline push CLI_MyPipeline
```

> Create tokens in the **pipeline's own project** (e.g., `CLI_MyPipeline`), not the parent. Use `teamcity pipeline list` to find the pipeline project ID.
>
{style="tip"}

## Pipelines in other commands

### Project tree

`teamcity project tree` shows pipelines alongside jobs, marked with `⬡`:

```Shell
teamcity project tree CLI
```

```
CLI CLI
├── CI CLI_CiCd ⬡ pipeline · 6 jobs
└── Build CLI_Build
```

Pipeline-generated sub-projects and virtual build types are hidden automatically.

### Run view

When viewing a pipeline run, the CLI shows the pipeline name and lists all jobs:

```Shell
teamcity run view 5800
```

```
✓ CI ⬡ 5800  #727 · main
Triggered by vcs · 3h ago · Took 8m 17s

Pipeline Jobs:
  lint                 Lint (build 5801)
  test_linux           Test Linux (build 5803)
  goreleaser           GoReleaser (build 5806)
```

### Job list

`teamcity job list` hides pipeline head build types by default. Use `--all` to show them:

```Shell
teamcity job list --project CLI         # hides pipeline heads
teamcity job list --project CLI --all   # shows everything
```

### Running a pipeline

Pipelines are triggered through the standard `run start` command using the pipeline head build type ID:

```Shell
teamcity run start CLI_CiCd
teamcity run start CLI_CiCd --watch
```

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-managing-runs.md">Managing runs</a>
        <a href="teamcity-cli-managing-jobs.md">Managing jobs</a>
        <a href="teamcity-cli-managing-projects.md">Managing projects</a>
    </category>
</seealso>
