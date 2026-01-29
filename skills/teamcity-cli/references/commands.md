# Command Reference

## Authentication (`tc auth`)

| Command                  | Description                       |
|--------------------------|-----------------------------------|
| `tc auth login -s <url>` | Authenticate with TeamCity server |
| `tc auth logout`         | Log out from current server       |
| `tc auth status`         | Show auth status and server info  |

## Builds/Runs (`tc run`)

| Command                    | Description              |
|----------------------------|--------------------------|
| `tc run list`              | List recent builds       |
| `tc run view <id>`         | View build details       |
| `tc run start <job-id>`    | Start a new build        |
| `tc run cancel <id>`       | Cancel a build           |
| `tc run restart <id>`      | Restart a build          |
| `tc run watch <id>`        | Watch build in real-time |
| `tc run log <id>`          | View build log           |
| `tc run tests <id>`        | View test results        |
| `tc run changes <id>`      | View VCS changes         |
| `tc run download <id>`     | Download artifacts       |
| `tc run pin <id>`          | Pin build                |
| `tc run unpin <id>`        | Unpin build              |
| `tc run tag <id> <tags>`   | Add tags                 |
| `tc run untag <id> <tags>` | Remove tags              |
| `tc run comment <id>`      | Manage comments          |

### Flags for `tc run list`

- `-j, --job <id>` - Filter by job
- `-b, --branch <name>` - Filter by branch
- `--status <status>` - Filter: success, failure, running
- `-u, --user <name>` - Filter by user
- `-p, --project <id>` - Filter by project
- `-n, --limit <n>` - Limit results
- `--since <time>` - Since time (e.g., 24h, 2024-01-01)
- `--json` - JSON output

### Flags for `tc run start`

- `-b, --branch <name>` - Branch to build
- `-P, --param <k=v>` - Build parameter (repeatable)
- `-S, --system <k=v>` - System property (repeatable)
- `-E, --env <k=v>` - Environment variable (repeatable)
- `-t, --tag <tag>` - Add tag (repeatable)
- `--watch` - Watch after starting
- `--clean` - Clean checkout
- `--agent <id>` - Run on specific agent
- `--personal` - Run as personal build
- `-l, --local-changes` - Include local changes (git, -, or path)
- `--rebuild-deps` - Rebuild all dependencies
- `--rebuild-failed-deps` - Rebuild failed/incomplete dependencies
- `--top` - Add to top of queue
- `-n, --dry-run` - Show what would be triggered without running

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

## Projects (`tc project`)

| Command                                  | Description             |
|------------------------------------------|-------------------------|
| `tc project list`                        | List projects           |
| `tc project view <id>`                   | View project details    |
| `tc project param list <id>`             | List parameters         |
| `tc project param get <id> <name>`       | Get parameter           |
| `tc project param set <id> <name> <val>` | Set parameter           |
| `tc project param delete <id> <name>`    | Delete parameter        |
| `tc project token put <id>`              | Store secret, get token |
| `tc project token get <id> <token>`      | Retrieve secret         |

## Queue (`tc queue`)

| Command                 | Description           |
|-------------------------|-----------------------|
| `tc queue list`         | List queued builds    |
| `tc queue remove <id>`  | Remove from queue     |
| `tc queue top <id>`     | Move to top of queue  |
| `tc queue approve <id>` | Approve waiting build |

## Direct API (`tc api`)

For features not covered by specific commands:

```bash
# GET request
tc api /app/rest/server

# POST request
tc api /app/rest/buildQueue -X POST -f 'buildType=id:MyBuild'

# With pagination
tc api /app/rest/builds --paginate --slurp
```

### Flags

- `-X, --method <method>` - HTTP method
- `-H, --header <h>` - Custom header (repeatable)
- `-f, --field <k=v>` - Body field (builds JSON)
- `--input <file>` - Read body from file
- `--paginate` - Fetch all pages
- `--slurp` - Combine pages into array

## Agents (`tc agent`)

| Command                    | Description                          |
|----------------------------|--------------------------------------|
| `tc agent list`            | List build agents                    |
| `tc agent view <id>`       | View agent details                   |
| `tc agent authorize <id>`  | Authorize agent to run builds        |
| `tc agent deauthorize <id>`| Revoke agent authorization           |
| `tc agent enable <id>`     | Enable agent                         |
| `tc agent disable <id>`    | Disable agent                        |
| `tc agent move <id> <pool>`| Move agent to different pool         |
| `tc agent jobs <id>`       | List compatible/incompatible jobs    |

### Flags for `tc agent list`

- `-p, --pool <name>` - Filter by agent pool
- `--connected` - Show only connected agents
- `--enabled` - Show only enabled agents
- `--authorized` - Show only authorized agents
- `-n, --limit <n>` - Limit results
- `--json` - JSON output

### Flags for `tc agent jobs`

- `--incompatible` - Show incompatible jobs with reasons
- `--json` - JSON output

## Agent Pools (`tc pool`)

| Command                          | Description                     |
|----------------------------------|---------------------------------|
| `tc pool list`                   | List agent pools                |
| `tc pool view <id>`              | View pool details               |
| `tc pool link <id> <project>`    | Link project to pool            |
| `tc pool unlink <id> <project>`  | Unlink project from pool        |

## Global Flags

Available on all commands:

- `--no-color` - Disable colored output
- `-q, --quiet` - Suppress non-essential output
- `--verbose` - Show detailed output including debug info
- `--no-input` - Disable interactive prompts
- `-w, --web` - Open in browser (on view commands)
