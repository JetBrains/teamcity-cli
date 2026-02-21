[//]: # (title: TeamCity CLI REST API Access)

<show-structure for="chapter" depth="2"/>

The `teamcity api` command lets you make authenticated HTTP requests to the [TeamCity REST API](https://www.jetbrains.com/help/teamcity/rest-api.html) directly from the command line. This is useful for accessing API endpoints not yet covered by dedicated CLI commands, building custom scripts, and debugging.

## Basic usage

The endpoint argument is the path portion of the URL, starting with `/app/rest/`:

```Shell
teamcity api /app/rest/server
```

The CLI automatically adds the base URL and authentication headers based on your current authentication context.

## HTTP methods

By default, requests use the GET method. Specify a different method with `-X`:

```Shell
# GET (default)
teamcity api /app/rest/projects

# POST
teamcity api /app/rest/buildQueue -X POST -f 'buildType=id:MyBuild'

# PUT
teamcity api /app/rest/builds/12345/comment -X PUT --input comment.txt

# DELETE
teamcity api /app/rest/builds/12345/tags/obsolete -X DELETE
```

## Sending data

### JSON fields

Use `-f` to build a JSON request body from key-value pairs:

```Shell
teamcity api /app/rest/buildQueue -X POST -f 'buildType=id:MyBuild'
teamcity api /app/rest/buildQueue -X POST -f 'buildType=id:MyBuild' -f 'branchName=main'
```

### Request body from a file

Use `--input` to read the request body from a file:

```Shell
teamcity api /app/rest/projects -X POST --input project.json
```

Read from stdin with `--input -`:

```Shell
echo '{"name": "New Project"}' | teamcity api /app/rest/projects -X POST --input -
```

## Custom headers

Add custom headers with `-H`:

```Shell
teamcity api /app/rest/builds -H "Accept: application/xml"
```

## Response handling

### Include response headers

```Shell
teamcity api /app/rest/server -i
```

### Raw output

Output the response without formatting:

```Shell
teamcity api /app/rest/server --raw
```

### Silent mode

Suppress output on success (useful in scripts where you only care about the exit code):

```Shell
teamcity api /app/rest/builds/12345/tags/release -X POST --silent
```

## Pagination

The TeamCity REST API returns paginated results for large collections. Use `--paginate` to automatically fetch all pages:

```Shell
teamcity api /app/rest/builds --paginate
```

Combine paginated results into a single JSON array with `--slurp`:

```Shell
teamcity api /app/rest/builds --paginate --slurp
```

> The `--slurp` flag requires `--paginate`. It collects items from all pages and outputs them as a single JSON array.
>
{style="note"}

## Examples

```Shell
# Get current user info
teamcity api /app/rest/users/current

# List all VCS roots
teamcity api /app/rest/vcs-roots

# Get build statistics
teamcity api /app/rest/builds/12345/statistics

# Trigger a build with parameters
teamcity api /app/rest/buildQueue -X POST \
  --input <(echo '{"buildType":{"id":"MyBuild"},"properties":{"property":[{"name":"version","value":"1.0"}]}}')

# Download a specific artifact
teamcity api /app/rest/builds/12345/artifacts/content/report.html --raw > report.html
```

## api flags

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

`-X`, `--method`

</td>
<td>

HTTP method (GET, POST, PUT, DELETE, PATCH). Default: GET.

</td>
</tr>
<tr>
<td>

`-f`, `--field`

</td>
<td>

Add a body field as `key=value`. Builds a JSON object. Can be repeated.

</td>
</tr>
<tr>
<td>

`-H`, `--header`

</td>
<td>

Add a custom header. Can be repeated.

</td>
</tr>
<tr>
<td>

`--input`

</td>
<td>

Read request body from a file. Use `-` for stdin.

</td>
</tr>
<tr>
<td>

`-i`, `--include`

</td>
<td>

Include response headers in output

</td>
</tr>
<tr>
<td>

`--raw`

</td>
<td>

Output raw response without formatting

</td>
</tr>
<tr>
<td>

`--silent`

</td>
<td>

Suppress output on success

</td>
</tr>
<tr>
<td>

`--paginate`

</td>
<td>

Automatically fetch all pages

</td>
</tr>
<tr>
<td>

`--slurp`

</td>
<td>

Combine paginated results into a JSON array (requires `--paginate`)

</td>
</tr>
</table>

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-scripting.md">Scripting and automation</a>
    </category>
</seealso>
