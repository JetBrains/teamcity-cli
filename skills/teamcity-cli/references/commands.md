# Command Reference

## Authentication (`teamcity auth`)

| Command                        | Description                       |
|--------------------------------|-----------------------------------|
| `teamcity auth login -s <url>` | Authenticate with TeamCity server |
| `teamcity auth logout`         | Log out from current server       |
| `teamcity auth status`         | Show auth status and server info  |

Login options:
- `-s, --server <url>` - TeamCity server URL
- `-t, --token <token>` - Access token
- `--insecure-storage` - Store token in plain text config file instead of system keyring

Environment override note:
- `TEAMCITY_URL` + `TEAMCITY_TOKEN` should be set together when overriding auth in scripts
- `TEAMCITY_URL` alone bypasses stored `teamcity auth login` credentials
- `TEAMCITY_HEADER_*` adds an HTTP header to every request: `TEAMCITY_HEADER_FOO_BAR=baz` sends `Foo-Bar: baz`. Use this for proxies that gate access (Cloudflare Access, Google IAP). Values are redacted in `--verbose` output.

## Builds/Runs (`teamcity run`)

| Command                          | Description              |
|----------------------------------|--------------------------|
| `teamcity run list`              | List recent builds       |
| `teamcity run view <id>`         | View build details       |
| `teamcity run start <job-id>`    | Start a new build        |
| `teamcity run cancel <id>`       | Cancel a build           |
| `teamcity run restart <id>`      | Restart a build          |
| `teamcity run watch <id>`        | Watch build in real-time |
| `teamcity run log <id>`          | View build log           |
| `teamcity run tests <id>`        | View test results        |
| `teamcity run changes <id>`      | View VCS changes         |
| `teamcity run artifacts <id>`    | List artifacts           |
| `teamcity run download <id>`     | Download artifacts       |
| `teamcity run pin <id>`          | Pin build                |
| `teamcity run unpin <id>`        | Unpin build              |
| `teamcity run tag <id> <tags>`   | Add tags                 |
| `teamcity run untag <id> <tags>` | Remove tags              |
| `teamcity run comment <id>`      | Manage comments          |
| `teamcity run tree <id>`        | Show snapshot dependency tree for a run |

### Flags for `teamcity run list`

Shows all branches and all build states (including canceled, personal, composite sub-builds) by default — matching the TeamCity UI. Use `--branch` to narrow to a specific branch, or `--branch @this` to use the current git branch.

- `-j, --job <id>` - Filter by job
- `-b, --branch <name>` - Filter by branch (`@this` = current git branch)
- `--status <status>` - Filter: success, failure, running, queued, error, unknown
- `-u, --user <name>` - Filter by user
- `--favorites` - Show favorite builds for the current user
- `-p, --project <id>` - Filter by project
- `-n, --limit <n>` - Limit results (default: 30)
- `--since <time>` - Since time (e.g., 24h, 7d, 2w, 2026-01-01)
- `--until <time>` - Until time (e.g., 12h, 7d, 2026-01-02)
- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)
- `--plain` - Plain text output for scripting
- `--no-header` - Omit header row (use with --plain)
- `-w, --web` - Open in browser

### Flags for `teamcity run start`

- `-b, --branch <name>` - Branch to build
- `--revision <sha>` - Pin build to a specific Git commit SHA
- `-P, --param <k=v>` - Build parameter (repeatable)
- `-S, --system <k=v>` - System property (repeatable)
- `-E, --env <k=v>` - Environment variable (repeatable)
- `-t, --tag <tag>` - Add tag (repeatable)
- `-m, --comment <text>` - Run comment
- `--watch` - Watch after starting
- `-i, --interval <s>` - Refresh interval in seconds when watching (default: 3)
- `--timeout <duration>` - Timeout when watching (e.g., 30m, 1h); implies --watch
- `--clean` - Clean checkout
- `--agent <id>` - Run on specific agent
- `--personal` - Run as personal build
- `-l, --local-changes` - Include local changes (git, -, or path)
- `--no-push` - Skip auto-push of branch to remote
- `--rebuild-deps` - Rebuild all dependencies
- `--rebuild-failed-deps` - Rebuild failed/incomplete dependencies
- `--reuse-deps <id,...>` - Reuse existing builds as snapshot dependencies (comma-separated IDs)
- `--top` - Add to top of queue
- `--dry-run` - Show what would be triggered without running
- `--json` - Output as JSON (for scripting)
- `-w, --web` - Open run in browser

### Flags for `teamcity run log`

- `--failed` - Show failure summary (problems and failed tests)
- `-j, --job <id>` - Get log for latest run of this job
- `-f, --follow` - Stream log output in real-time until build finishes
- `--tail <N>` - Show last N log messages
- `--raw` - Show raw log without formatting
- `--json` - Output as JSON
- `-w, --web` - Open build log in browser

### Flags for `teamcity run watch`

- `-i, --interval <s>` - Refresh interval in seconds
- `--logs` - Stream build logs while watching
- `--quiet` - Minimal output, show only state changes and result
- `--json` - Wait for completion and output result as JSON
- `--timeout <duration>` - Timeout duration (e.g., 30m, 1h)

### Flags for `teamcity run view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

### Flags for `teamcity run tests`

- `--failed` - Show only failed tests
- `-j, --job <id>` - Get tests for latest run of this job
- `--json` - Output as JSON
- `-n, --limit <n>` - Maximum number of tests to show

### Flags for `teamcity run changes`

- `--json` - Output as JSON
- `--no-files` - Hide file list, show commits only

### Flags for `teamcity run artifacts`

- `-j, --job <id>` - List artifacts from latest run of this job
- `-p, --path <subdir>` - Browse artifacts under this subdirectory
- `--json` - Output as JSON

### Flags for `teamcity run download`

- `-a, --artifact <pattern>` - Artifact name pattern to filter (matches full path and basename)
- `-p, --path <subdir>` - Download artifacts under this subdirectory
- `-o, --output <path>` - Local directory to save artifacts to

### Flags for `teamcity run cancel`

- `--comment <text>` - Comment for cancellation
- `-y, --yes` - Skip confirmation prompt

### Flags for `teamcity run restart`

- `--watch` - Watch the new run after restarting
- `-i, --interval <s>` - Refresh interval in seconds when watching (default: 3)
- `--timeout <duration>` - Timeout when watching (e.g., 30m, 1h); implies --watch
- `-w, --web` - Open run in browser

### Flags for `teamcity run pin`

- `-m, --comment <text>` - Comment explaining why the run is pinned

### Flags for `teamcity run comment`

- `--delete` - Delete the comment

### Flags for `teamcity run tree`

- `-d, --depth <n>` - Limit tree depth (0 = unlimited)
- `--json` - Output as JSON

## Jobs (`teamcity job`)

| Command                              | Description               |
|--------------------------------------|---------------------------|
| `teamcity job list`                        | List build configurations      |
| `teamcity job view <id>`                   | View job details               |
| `teamcity job tree <id>`                   | Show snapshot dependency tree  |
| `teamcity job pause <id>`                  | Pause job                      |
| `teamcity job resume <id>`                 | Resume job                     |
| `teamcity job param list <id>`             | List parameters                |
| `teamcity job param get <id> <name>`       | Get parameter                  |
| `teamcity job param set <id> <name> <val>` | Set parameter                  |
| `teamcity job param delete <id> <name>`    | Delete parameter               |

### Flags for `teamcity job list`

- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)
- `-n, --limit <n>` - Maximum number of jobs
- `-p, --project <id>` - Filter by project ID

### Flags for `teamcity job view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

### Flags for `teamcity job tree`

- `-d, --depth <n>` - Limit tree depth (0 = unlimited)
- `--only <type>` - Show only `dependents` or `dependencies`

### Flags for `teamcity job param list`

- `--json` - Output as JSON

### Flags for `teamcity job param set`

- `--secure` - Mark as secure/password parameter

## Projects (`teamcity project`)

| Command                                        | Description                  |
|------------------------------------------------|------------------------------|
| `teamcity project list`                        | List projects                |
| `teamcity project view <id>`                   | View project details         |
| `teamcity project create <name>`               | Create a project             |
| `teamcity project tree [id]`                   | Show project hierarchy tree  |
| `teamcity project vcs list --project <id>`     | List VCS roots               |
| `teamcity project vcs view <id>`              | View VCS root details        |
| `teamcity project vcs create --project <id>`  | Open browser to add VCS root |
| `teamcity project vcs delete <id>`            | Delete a VCS root            |
| `teamcity project param list <id>`             | List parameters              |
| `teamcity project param get <id> <name>`       | Get parameter                |
| `teamcity project param set <id> <name> <val>` | Set parameter                |
| `teamcity project param delete <id> <name>`    | Delete parameter             |
| `teamcity project token put <id>`              | Store secret, get token      |
| `teamcity project token get <id> <token>`      | Retrieve secret              |
| `teamcity project settings export <id>`        | Export settings as ZIP       |
| `teamcity project settings status <id>`        | Show versioned settings sync |
| `teamcity project settings validate [path]`    | Validate Kotlin DSL config   |

### Flags for `teamcity project tree`

- `-d, --depth <n>` - Limit tree depth (0 = unlimited)
- `--no-jobs` - Hide build configurations

### Flags for `teamcity project list`

- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)
- `-n, --limit <n>` - Maximum number of projects
- `-p, --parent <id>` - Filter by parent project ID

### Flags for `teamcity project view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

### Flags for `teamcity project create`

- `--id <id>` - Explicit project ID (default: auto-generated from name)
- `-p, --parent <id>` - Parent project ID (default: `_Root`)
- `--json` - Output as JSON
- `-w, --web` - Open in browser after creation

### Flags for `teamcity project vcs list`

- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)
- `-n, --limit <n>` - Maximum number of VCS roots
- `-p, --project <id>` - Project ID (required)

### Flags for `teamcity project vcs view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

### Flags for `teamcity project vcs delete`

- `-y, --yes` - Skip confirmation prompt

### Flags for `teamcity project param list`

- `--json` - Output as JSON

### Flags for `teamcity project param set`

- `--secure` - Mark as secure/password parameter

### Flags for `teamcity project settings export`

- `--kotlin` - Export as Kotlin DSL (default)
- `--xml` - Export as XML
- `-o, --output <path>` - Output file path (default: projectSettings.zip)
- `--relative-ids` - Use relative IDs in exported settings

### Flags for `teamcity project settings status`

- `--json` - Output as JSON

### Flags for `teamcity project settings validate`

- `--verbose` - Show full Maven output
- Positional argument: optional filesystem path to `.teamcity` (not a project ID/name; there is no `--dir` flag)

### Flags for `teamcity project token put`

- `--stdin` - Read value from stdin

## Queue (`teamcity queue`)

| Command                       | Description           |
|-------------------------------|-----------------------|
| `teamcity queue list`         | List queued builds    |
| `teamcity queue remove <id>`  | Remove from queue     |
| `teamcity queue top <id>`     | Move to top of queue  |
| `teamcity queue approve <id>` | Approve waiting build |

### Flags for `teamcity queue list`

- `-j, --job <id>` - Filter by job ID
- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)
- `-n, --limit <n>` - Maximum number of queued runs

### Flags for `teamcity queue remove`

- `-y, --yes` - Skip confirmation prompt

## Agents (`teamcity agent`)

| Command                           | Description                       |
|-----------------------------------|-----------------------------------|
| `teamcity agent list`             | List build agents                 |
| `teamcity agent view <id>`        | View agent details                |
| `teamcity agent authorize <id>`   | Authorize agent to run builds     |
| `teamcity agent deauthorize <id>` | Revoke agent authorization        |
| `teamcity agent enable <id>`      | Enable agent                      |
| `teamcity agent disable <id>`     | Disable agent                     |
| `teamcity agent move <id> <pool>` | Move agent to different pool      |
| `teamcity agent jobs <id>`        | List compatible/incompatible jobs |
| `teamcity agent exec <id> <cmd>`  | Execute command on agent          |
| `teamcity agent term <id>`        | Open interactive shell on agent   |
| `teamcity agent reboot <id>`      | Reboot a build agent              |

### Flags for `teamcity agent list`

- `-p, --pool <name>` - Filter by agent pool
- `--connected` - Show only connected agents
- `--enabled` - Show only enabled agents
- `--authorized` - Show only authorized agents
- `-n, --limit <n>` - Limit results
- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)

### Flags for `teamcity agent view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

### Flags for `teamcity agent jobs`

- `--incompatible` - Show incompatible jobs with reasons
- `--json` - Output as JSON

### Flags for `teamcity agent exec`

- `--timeout <duration>` - Command timeout

### Flags for `teamcity agent reboot`

- `--graceful` - Wait for current build to finish before rebooting
- `-y, --yes` - Skip confirmation prompt

## Agent Pools (`teamcity pool`)

| Command                          | Description              |
|----------------------------------|--------------------------|
| `teamcity pool list`                   | List agent pools         |
| `teamcity pool view <id>`              | View pool details        |
| `teamcity pool link <id> <project>`    | Link project to pool     |
| `teamcity pool unlink <id> <project>`  | Unlink project from pool |

### Flags for `teamcity pool list`

- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)

### Flags for `teamcity pool view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

## Pipelines (`teamcity pipeline`)

Pipelines are YAML-first build configurations. Each pipeline is a project that can contain multiple jobs defined in a `.teamcity.yml` file. Pipelines differ from jobs/build configs: they use YAML for configuration and can be stored in VCS or on the server.

| Command                                  | Description                              |
|------------------------------------------|------------------------------------------|
| `teamcity pipeline list`                 | List pipelines                           |
| `teamcity pipeline view <id>`            | View pipeline details                    |
| `teamcity pipeline create <name>`        | Create pipeline from YAML                |
| `teamcity pipeline validate [file]`      | Validate pipeline YAML against schema    |
| `teamcity pipeline pull <id>`            | Download pipeline YAML                   |
| `teamcity pipeline push <id> [file]`     | Upload pipeline YAML                     |
| `teamcity pipeline delete <id>`          | Delete a pipeline                        |

### Flags for `teamcity pipeline list`

- `-p, --project <id>` - Filter by project ID
- `-n, --limit <n>` - Maximum number of items (default: 30)
- `--json` - JSON output (use `--json=` to list fields, `--json=f1,f2` for specific)
- `--plain` - Plain text output for scripting
- `--no-header` - Omit header row

### Flags for `teamcity pipeline view`

- `--json` - Output as JSON
- `-w, --web` - Open in browser

### Flags for `teamcity pipeline create`

- `-p, --project <id>` - Parent project ID **(required)**
- `--vcs-root <id>` - VCS root ID (interactive selection if omitted)
- `-f, --file <path>` - Pipeline YAML file (default: `.teamcity.yml`)

### Flags for `teamcity pipeline validate`

- `--schema <path>` - Local JSON schema file (overrides server schema)
- `--refresh-schema` - Force re-fetch schema from server

### Flags for `teamcity pipeline pull`

- `-o, --output <path>` - Write YAML to file instead of stdout

### Flags for `teamcity pipeline delete`

- `-y, --yes` - Skip confirmation prompt

## Configuration (`teamcity config`)

| Command                               | Description                    |
|---------------------------------------|--------------------------------|
| `teamcity config list`                | List all configuration values  |
| `teamcity config get <key>`           | Get a configuration value      |
| `teamcity config set <key> <value>`   | Set a configuration value      |

Valid keys: `default_server`, `guest`, `ro`, `token_expiry`.

Per-server keys (`guest`, `ro`, `token_expiry`) use `--server <url>` to target a specific server. Without `--server`, the default server is used.

### Flags for `teamcity config list`

- `--json` - Output as JSON

### Flags for `teamcity config get` and `set`

- `-s, --server <url>` - Server URL for per-server settings

### Examples

```bash
# Switch default server
teamcity config set default_server tc.example.com

# Enable read-only mode
teamcity config set ro true --server tc.example.com

# Check current default server
teamcity config get default_server
```

## Direct API (`teamcity api`)

For features not covered by specific commands. Endpoints always start with `/app/rest/`.
Pass only the endpoint path as the first argument (never include `GET`, `POST`, etc. in the path).

```bash
# GET request
teamcity api '/app/rest/server'

# POST request
teamcity api '/app/rest/buildQueue' -X POST -f 'buildType=id:MyBuild'

# With pagination
teamcity api '/app/rest/builds' --paginate --slurp

# Browse artifact subdirectory
teamcity api '/app/rest/builds/id:BUILD_ID/artifacts/children/SUBPATH'
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

## List Output Flags

Available on all list commands (`run list`, `agent list`, `job list`, `pool list`, `project list`, `queue list`, `project vcs list`, `pipeline list`) and on `agent jobs`, `project param list`, `job param list`:

- `--plain` - Tab-separated plain text output for scripting (mutually exclusive with `--json`)
- `--no-header` - Omit header row (use with `--plain`)
