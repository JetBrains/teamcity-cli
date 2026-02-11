# Command Reference

## Authentication (`tc auth`)

| Command                  | Description                       |
|--------------------------|-----------------------------------|
| `tc auth login -s <url>` | Authenticate with TeamCity server |
| `tc auth logout`         | Log out from current server       |
| `tc auth status`         | Show auth status and server info  |

Login options:
- `-s, --server <url>` - TeamCity server URL
- `-t, --token <token>` - Access token

## Builds/Runs (`tc run`)

| Command                    | Description                |
|----------------------------|----------------------------|
| `tc run list`              | List recent builds         |
| `tc run view <id>`         | View build details         |
| `tc run start <job-id>`    | Start a new build          |
| `tc run cancel <id>`       | Cancel a build             |
| `tc run restart <id>`      | Restart a build            |
| `tc run watch <id>`        | Watch build in real-time   |
| `tc run log <id>`          | View build log             |
| `tc run tests <id>`        | View test results          |
| `tc run changes <id>`      | View VCS changes           |
| `tc run artifacts <id>`    | List artifacts             |
| `tc run download <id>`     | Download artifacts         |
| `tc run pin <id>`          | Pin build                  |
| `tc run unpin <id>`        | Unpin build                |
| `tc run tag <id> <tags>`   | Add tags                   |
| `tc run untag <id> <tags>` | Remove tags                |
| `tc run comment <id>`      | Manage comments            |

### Flags for `tc run list`

- `-j, --job <id>` - Filter by job
- `-b, --branch <name>` - Filter by branch
- `--status <status>` - Filter: success, failure, running
- `-u, --user <name>` - Filter by user
- `-p, --project <id>` - Filter by project
- `-n, --limit <n>` - Limit results
- `--since <time>` - Since time (e.g., 24h, 2026-01-01)
- `--until <time>` - Until time (e.g., 12h, 2026-01-02)
- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)
- `--plain` - Plain text output for scripting
- `--no-header` - Omit header row (use with --plain)
- `-w, --web` - Open in browser

### Flags for `tc run start`

- `-b, --branch <name>` - Branch to build
- `-P, --param <k=v>` - Build parameter (repeatable)
- `-S, --system <k=v>` - System property (repeatable)
- `-E, --env <k=v>` - Environment variable (repeatable)
- `-t, --tag <tag>` - Add tag (repeatable)
- `-m, --comment <text>` - Run comment
- `--watch` - Watch after starting
- `--clean` - Clean checkout
- `--agent <id>` - Run on specific agent
- `--personal` - Run as personal build
- `-l, --local-changes` - Include local changes (git, -, or path)
- `--no-push` - Skip auto-push of branch to remote
- `--rebuild-deps` - Rebuild all dependencies
- `--rebuild-failed-deps` - Rebuild failed/incomplete dependencies
- `--top` - Add to top of queue
- `-n, --dry-run` - Show what would be triggered without running
- `--json` - Output as JSON (for scripting)
- `-w, --web` - Open run in browser

### Flags for `tc run log`

- `--failed` - Show failure summary (problems and failed tests)
- `-j, --job <id>` - Get log for latest run of this job
- `--raw` - Show raw log without formatting

### Flags for `tc run watch`

- `-i, --interval <s>` - Refresh interval in seconds
- `--logs` - Stream build logs while watching
- `-Q, --quiet` - Minimal output, show only state changes and result
- `--timeout <duration>` - Timeout duration (e.g., 30m, 1h)

### Flags for `tc run view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

### Flags for `tc run tests`

- `--failed` - Show only failed tests
- `-j, --job <id>` - Get tests for latest run of this job
- `--json` - Output as JSON
- `-n, --limit <n>` - Maximum number of tests to show

### Flags for `tc run changes`

- `--json` - Output as JSON
- `--no-files` - Hide file list, show commits only

### Flags for `tc run artifacts`

- `-j, --job <id>` - List artifacts from latest run of this job
- `--json` - Output as JSON

### Flags for `tc run download`

- `-a, --artifact <pattern>` - Artifact name pattern to download
- `-d, --dir <path>` - Directory to download artifacts to

### Flags for `tc run cancel`

- `--comment <text>` - Comment for cancellation
- `-f, --force` - Skip confirmation prompt

### Flags for `tc run restart`

- `--watch` - Watch the new run after restarting
- `-w, --web` - Open run in browser

### Flags for `tc run pin`

- `-m, --comment <text>` - Comment explaining why the run is pinned

### Flags for `tc run comment`

- `--delete` - Delete the comment

## Jobs (`tc job`)

| Command                              | Description               |
|--------------------------------------|---------------------------|
| `tc job list`                        | List build configurations |
| `tc job view <id>`                   | View job details          |
| `tc job pause <id>`                  | Pause job                 |
| `tc job resume <id>`                 | Resume job                |
| `tc job param list <id>`             | List parameters           |
| `tc job param get <id> <name>`       | Get parameter             |
| `tc job param set <id> <name> <val>` | Set parameter             |
| `tc job param delete <id> <name>`    | Delete parameter          |

### Flags for `tc job list`

- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)
- `-n, --limit <n>` - Maximum number of jobs
- `-p, --project <id>` - Filter by project ID

### Flags for `tc job view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

### Flags for `tc job param list`

- `--json` - Output as JSON

### Flags for `tc job param set`

- `--secure` - Mark as secure/password parameter

## Projects (`tc project`)

| Command                                  | Description                  |
|------------------------------------------|------------------------------|
| `tc project list`                        | List projects                |
| `tc project view <id>`                   | View project details         |
| `tc project param list <id>`             | List parameters              |
| `tc project param get <id> <name>`       | Get parameter                |
| `tc project param set <id> <name> <val>` | Set parameter                |
| `tc project param delete <id> <name>`    | Delete parameter             |
| `tc project token put <id>`              | Store secret, get token      |
| `tc project token get <id> <token>`      | Retrieve secret              |
| `tc project settings export <id>`        | Export settings as ZIP       |
| `tc project settings status <id>`        | Show versioned settings sync |
| `tc project settings validate`           | Validate Kotlin DSL config   |

### Flags for `tc project list`

- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)
- `-n, --limit <n>` - Maximum number of projects
- `-p, --parent <id>` - Filter by parent project ID

### Flags for `tc project view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

### Flags for `tc project param list`

- `--json` - Output as JSON

### Flags for `tc project param set`

- `--secure` - Mark as secure/password parameter

### Flags for `tc project settings export`

- `--kotlin` - Export as Kotlin DSL (default)
- `--xml` - Export as XML
- `-o, --output <path>` - Output file path (default: projectSettings.zip)
- `--relative-ids` - Use relative IDs in exported settings

### Flags for `tc project settings status`

- `--json` - Output as JSON

### Flags for `tc project settings validate`

- `-v, --verbose` - Show full Maven output

### Flags for `tc project token put`

- `--stdin` - Read value from stdin

## Queue (`tc queue`)

| Command                 | Description           |
|-------------------------|-----------------------|
| `tc queue list`         | List queued builds    |
| `tc queue remove <id>`  | Remove from queue     |
| `tc queue top <id>`     | Move to top of queue  |
| `tc queue approve <id>` | Approve waiting build |

### Flags for `tc queue list`

- `-j, --job <id>` - Filter by job ID
- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)
- `-n, --limit <n>` - Maximum number of queued runs

### Flags for `tc queue remove`

- `-f, --force` - Skip confirmation prompt

## Agents (`tc agent`)

| Command                     | Description                       |
|-----------------------------|-----------------------------------|
| `tc agent list`             | List build agents                 |
| `tc agent view <id>`        | View agent details                |
| `tc agent authorize <id>`   | Authorize agent to run builds     |
| `tc agent deauthorize <id>` | Revoke agent authorization        |
| `tc agent enable <id>`      | Enable agent                      |
| `tc agent disable <id>`     | Disable agent                     |
| `tc agent move <id> <pool>` | Move agent to different pool      |
| `tc agent jobs <id>`        | List compatible/incompatible jobs |
| `tc agent exec <id> <cmd>`  | Execute command on agent          |
| `tc agent term <id>`        | Open interactive shell on agent   |
| `tc agent reboot <id>`      | Reboot a build agent              |

### Flags for `tc agent list`

- `-p, --pool <name>` - Filter by agent pool
- `--connected` - Show only connected agents
- `--enabled` - Show only enabled agents
- `--authorized` - Show only authorized agents
- `-n, --limit <n>` - Limit results
- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)

### Flags for `tc agent view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

### Flags for `tc agent jobs`

- `--incompatible` - Show incompatible jobs with reasons
- `--json` - Output as JSON

### Flags for `tc agent exec`

- `--timeout <duration>` - Command timeout

### Flags for `tc agent reboot`

- `--after-build` - Wait for current build to finish before rebooting
- `-y, --yes` - Skip confirmation prompt

## Agent Pools (`tc pool`)

| Command                          | Description              |
|----------------------------------|--------------------------|
| `tc pool list`                   | List agent pools         |
| `tc pool view <id>`              | View pool details        |
| `tc pool link <id> <project>`    | Link project to pool     |
| `tc pool unlink <id> <project>`  | Unlink project from pool |

### Flags for `tc pool list`

- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)

### Flags for `tc pool view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

## Direct API (`tc api`)

For features not covered by specific commands. Endpoints always start with `/app/rest/`.

```bash
# GET request
tc api /app/rest/server

# POST request
tc api /app/rest/buildQueue -X POST -f 'buildType=id:MyBuild'

# With pagination
tc api /app/rest/builds --paginate --slurp

# Browse artifact subdirectory
tc api /app/rest/builds/id:BUILD_ID/artifacts/children/SUBPATH
```

### Flags

- `-X, --method <method>` - HTTP method
- `-H, --header <h>` - Custom header (repeatable)
- `-f, --field <k=v>` - Body field (builds JSON)
- `--input <file>` - Read body from file (use - for stdin)
- `--paginate` - Fetch all pages
- `--slurp` - Combine pages into array (requires --paginate)
- `--raw` - Output raw response without formatting
- `--silent` - Suppress output on success
- `-i, --include` - Include response headers in output

## Global Flags

Available on all commands:

- `-h, --help` - Help for command
- `-v, --version` - Version information
- `--no-color` - Disable colored output
- `-q, --quiet` - Suppress non-essential output
- `--verbose` - Show detailed output including debug info
- `--no-input` - Disable interactive prompts
- `-w, --web` - Open in browser (on view commands)
