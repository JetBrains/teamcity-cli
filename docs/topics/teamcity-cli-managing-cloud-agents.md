[//]: # (title: Managing Cloud Agents)

<show-structure for="chapter" depth="2"/>

Cloud agent profiles provision build agents on demand from cloud providers (AWS, Azure, GCP, Kubernetes, and others). Profiles belong to TeamCity projects, their images define what to provision, and instances are the running cloud machines. The `teamcity project cloud` command group lets you inspect and manage all three.

> For background on how cloud integrations work in TeamCity, see [TeamCity Integration with Cloud Solutions](https://www.jetbrains.com/help/teamcity/teamcity-integration-with-cloud-solutions.html).
>
{style="note"}

## Cloud profiles

### Listing profiles

View cloud profiles configured for a project:

```Shell
teamcity project cloud profile list
```

<img src="cloud-profile-list.gif" alt="Listing cloud profiles and viewing details" border-effect="rounded"/>

List profiles for a specific project:

```Shell
teamcity project cloud profile list --project MyProject
teamcity project cloud profile list --project MyProject --json
```

#### profile list flags

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

Maximum number of profiles to display

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

### Viewing profile details

```Shell
teamcity project cloud profile view amazon-1
teamcity project cloud profile view amazon-1 --json
```

## Cloud images

### Listing images

View cloud images available for a project:

```Shell
teamcity project cloud image list --project MyProject
```

<img src="cloud-instance-list.gif" alt="Listing cloud images and instances" border-effect="rounded"/>

Filter by cloud profile:

```Shell
teamcity project cloud image list --project MyProject --profile aws-prod
```

#### image list flags

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

`--profile`

</td>
<td>

Filter images by cloud profile ID

</td>
</tr>
<tr>
<td>

`-n`, `--limit`

</td>
<td>

Maximum number of images to display

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

### Viewing image details

```Shell
teamcity project cloud image view ubuntu-22-large
teamcity project cloud image view ubuntu-22-large --json
```

### Starting an instance from an image

Start a new cloud instance from a selected image:

```Shell
teamcity project cloud image start ubuntu-22-large
teamcity project cloud image start ubuntu-22-large --json
```

The command returns the new instance ID on success.

## Cloud instances

### Listing instances

View running cloud instances for a project:

```Shell
teamcity project cloud instance list --project MyProject
```

Filter by image:

```Shell
teamcity project cloud instance list --project MyProject --image ubuntu-22-large
```

#### instance list flags

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

`--image`

</td>
<td>

Filter instances by cloud image name or explicit ID locator

</td>
</tr>
<tr>
<td>

`-n`, `--limit`

</td>
<td>

Maximum number of instances to display

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

### Viewing instance details

```Shell
teamcity project cloud instance view i-0245b46070c443201
teamcity project cloud instance view i-0245b46070c443201 --json
```

### Stopping an instance

Stop a cloud instance gracefully:

```Shell
teamcity project cloud instance stop i-0245b46070c443201
```

Force-stop for immediate termination:

```Shell
teamcity project cloud instance stop i-0245b46070c443201 --force
```

> Bare image selectors are matched by name. If the name is ambiguous, use the explicit ID locator from `list` or `--json` output (for example `id:my-image,profileId:aws-1`).
>
{style="tip"}

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-managing-agents.md">Managing agents</a>
        <a href="teamcity-cli-managing-agent-pools.md">Managing agent pools</a>
        <a href="teamcity-cli-managing-projects.md">Managing projects</a>
    </category>
</seealso>
