# Output Formats

Most commands support multiple output formats for different use cases.

## Available Formats

| Format          | Flag          | Use Case                       |
|-----------------|---------------|--------------------------------|
| Table (default) | none          | Human-readable, colored output |
| Plain text      | `--plain`     | Scripting, parsing             |
| JSON            | `--json`      | Programmatic access            |
| No color        | `--no-color`  | Logs, CI environments          |
| No header       | `--no-header` | Clean output for piping        |

## JSON Output

**Default JSON (all fields):**
```bash
tc run list --json
```

**List available fields:**
```bash
tc run list --json=
```

**Select specific fields:**
```bash
tc run list --json=id,status,branch
```

**Nested fields (dot notation):**
```bash
tc run list --json=id,buildType.name,status
```

## Scripting Examples

**Get build IDs of failed builds:**
```bash
tc run list --status failure --plain --no-header | awk '{print $1}'
```

**JSON with jq:**
```bash
tc run list --json | jq '.[] | {id, status, branch}'
```

**Filter builds by pattern:**
```bash
tc run list --json | jq '.[] | select(.branch | contains("feature"))'
```

**Count builds by status:**
```bash
tc run list --json | jq 'group_by(.status) | map({status: .[0].status, count: length})'
```

## Environment Variables

For non-interactive use (CI/CD, scripts):

```bash
export TEAMCITY_URL="https://teamcity.example.com"
export TEAMCITY_TOKEN="your-api-token"

# Commands will use these automatically
tc run list
```

## Combining with Other Tools

**Open in browser:**
```bash
tc run view <id> -w
```

**Pipe to less with color:**
```bash
tc run list | less -R
```

**Watch and notify:**
```bash
tc run watch <id> && notify-send "Build complete"
```
