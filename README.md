# tc – TeamCity CLI

(unofficial) command-line interface for TeamCity that lets you manage builds, jobs, and projects without leaving your terminal.

<details>
<summary> TABLE OF CONTENTS </summary>


<!-- TOC -->
* [tc – TeamCity CLI](#tc--teamcity-cli)
  * [Why tc?](#why-tc)
  * [Installation](#installation)
  * [Quick Start](#quick-start)
  * [Commands](#commands)
  * [Authentication](#authentication)
    * [auth login](#auth-login)
    * [auth logout](#auth-logout)
    * [auth status](#auth-status)
  * [Runs](#runs)
    * [run cancel](#run-cancel)
    * [run changes](#run-changes)
    * [run comment](#run-comment)
    * [run download](#run-download)
    * [run list](#run-list)
    * [run log](#run-log)
    * [run pin](#run-pin)
    * [run restart](#run-restart)
    * [run start](#run-start)
    * [run tag](#run-tag)
    * [run tests](#run-tests)
    * [run unpin](#run-unpin)
    * [run untag](#run-untag)
    * [run view](#run-view)
    * [run watch](#run-watch)
  * [Jobs](#jobs)
    * [job list](#job-list)
    * [job param delete](#job-param-delete)
    * [job param get](#job-param-get)
    * [job param list](#job-param-list)
    * [job param set](#job-param-set)
    * [job pause](#job-pause)
    * [job resume](#job-resume)
    * [job view](#job-view)
  * [Projects](#projects)
    * [project list](#project-list)
    * [project param delete](#project-param-delete)
    * [project param get](#project-param-get)
    * [project param list](#project-param-list)
    * [project param set](#project-param-set)
    * [project token get](#project-token-get)
    * [project token put](#project-token-put)
    * [project view](#project-view)
  * [Queue](#queue)
    * [queue approve](#queue-approve)
    * [queue list](#queue-list)
    * [queue remove](#queue-remove)
    * [queue top](#queue-top)
  * [API](#api)
  * [Configuration](#configuration)
  * [Shell Completion](#shell-completion)
  * [Global Flags](#global-flags)
  * [License](#license)
<!-- TOC -->

</details>

## Why tc?

- **Stay in your terminal** – Start builds, check statuses, and view logs without switching to a browser
- **Scriptable automation** – Integrate TeamCity into shell scripts, CI pipelines, and git hooks with simple commands
- **Quick access to logs** – Stream build output or fetch logs with `tc run log`, no clicking through UI panels
- **Manage at scale** – List, filter, and batch-operate on runs and jobs across projects
- **JSON output** – Pipe to `jq` for custom filtering; use `--plain` for awk/grep-friendly output
- **Escape hatch available** – `tc api` command gives you direct REST API access when you need it

## Installation

```bash
go install github.com/tiulpin/teamcity-cli/tc@latest
```

Or build from source:

```bash
git clone https://github.com/tiulpin/teamcity-cli.git
cd teamcity-cli
go build -o tc ./tc
```

## Quick Start

```bash
# Authenticate with your TeamCity server
tc auth login

# List recent builds
tc run list --limit 10

# Start a build and watch it run
tc run start MyProject_Build --branch main --watch

# View logs from the latest build of a job
tc run log --job MyProject_Build

# Check what's in the queue
tc queue list
```

## Commands

| Command               | Description                         |
|-----------------------|-------------------------------------|
| **auth**              |                                     |
| `tc auth login`       | Authenticate with a TeamCity server |
| `tc auth logout`      | Log out from the current server     |
| `tc auth status`      | Show authentication status          |
| **run**               |                                     |
| `tc run list`         | List recent builds                  |
| `tc run start`        | Start a new build                   |
| `tc run view`         | View build details                  |
| `tc run watch`        | Watch a build in real-time          |
| `tc run log`          | View build log output               |
| `tc run changes`      | Show commits included in a build    |
| `tc run tests`        | Show test results                   |
| `tc run cancel`       | Cancel a running or queued build    |
| `tc run download`     | Download artifacts                  |
| `tc run restart`      | Restart with same configuration     |
| `tc run pin/unpin`    | Pin or unpin a build                |
| `tc run tag/untag`    | Add or remove tags                  |
| `tc run comment`      | Set or view build comment           |
| **job**               |                                     |
| `tc job list`         | List build configurations           |
| `tc job view`         | View job details                    |
| `tc job pause/resume` | Pause or resume a job               |
| `tc job param`        | Manage job parameters               |
| **project**           |                                     |
| `tc project list`     | List projects                       |
| `tc project view`     | View project details                |
| `tc project param`    | Manage project parameters           |
| `tc project token`    | Manage secure tokens                |
| **queue**             |                                     |
| `tc queue list`       | List queued builds                  |
| `tc queue approve`    | Approve a queued build              |
| `tc queue remove`     | Remove from queue                   |
| `tc queue top`        | Move to top of queue                |
| **api**               |                                     |
| `tc api`              | Make raw API requests               |

## Configuration

Configuration is stored in `~/.config/tc/config.yml`:

```yaml
default_server: https://teamcity.example.com
servers:
  https://teamcity.example.com:
    token: <your-token>
    user: username
```

### Multiple Servers

You can authenticate with multiple TeamCity servers. Each server's credentials are stored separately.

**Adding servers:**

```bash
# Log in to your first server
tc auth login --server https://teamcity1.example.com

# Log in to additional servers (becomes the new default)
tc auth login --server https://teamcity2.example.com
```

**Switching between servers:**

```bash
# Option 1: Use environment variable (recommended for scripts)
TEAMCITY_URL=https://teamcity1.example.com tc run list

# Option 2: Export for your session
export TEAMCITY_URL=https://teamcity1.example.com
tc run list  # uses teamcity1
tc auth status  # shows teamcity1

# Option 3: Log in again to change the default
tc auth login --server https://teamcity1.example.com
```

**Example multi-server config:**

```yaml
default_server: https://teamcity-prod.example.com
servers:
  https://teamcity-prod.example.com:
    token: <token>
    user: alice
  https://teamcity-staging.example.com:
    token: <token>
    user: alice
  https://teamcity-dev.example.com:
    token: <token>
    user: alice
```

**CI/CD usage:**

Environment variables always take precedence over config file settings:

```bash
export TEAMCITY_URL="https://teamcity.example.com"
export TEAMCITY_TOKEN="your-access-token"
tc run start MyProject_Build  # uses env vars, ignores config file
```

## Shell Completion

```bash
# Bash
tc completion bash > /etc/bash_completion.d/tc

# Zsh
tc completion zsh > "${fpath[1]}/_tc"

# Fish
tc completion fish > ~/.config/fish/completions/tc.fish

# PowerShell
tc completion powershell > tc.ps1
```

## Global Flags

- `-h, --help` – Help for command
- `-v, --version` – Version information
- `--no-color` – Disable colored output
- `-q, --quiet` – Suppress non-essential output
- `--verbose` – Show detailed output including debug info
- `--no-input` – Disable interactive prompts

<!-- COMMANDS_START -->

## Authentication

### auth login

Authenticate with a TeamCity server using an access token.

This will:
1. Prompt for your TeamCity server URL
2. Open your browser to generate an access token
3. Validate and store the token securely

**Options:**
- `-s, --server` – TeamCity server URL
- `-t, --token` – Access token

**Environment variables** (for CI/CD):

```bash
export TEAMCITY_URL="https://teamcity.example.com"
export TEAMCITY_TOKEN="your-access-token"
```

### auth logout

Log out from a TeamCity server

### auth status

Show authentication status

---

## Runs

### run cancel

Cancel a running or queued run.

```bash
tc run cancel 12345
tc run cancel 12345 --comment "Cancelling for hotfix"
tc run cancel 12345 --force
```

**Options:**
- `--comment` – Comment for cancellation
- `-f, --force` – Skip confirmation prompt

### run changes

Show the VCS changes (commits) included in a run.

```bash
tc run changes 12345
tc run changes 12345 --no-files
tc run changes 12345 --json
```

**Options:**
- `--json` – Output as JSON
- `--no-files` – Hide file list, show commits only

### run comment

Set, view, or delete a comment on a run.

```bash
tc run comment 12345
tc run comment 12345 "Deployed to production"
tc run comment 12345 --delete
```

**Options:**
- `--delete` – Delete the comment

### run download

Download artifacts from a completed run.

```bash
tc run download 12345
tc run download 12345 --dir ./artifacts
tc run download 12345 --artifact "*.jar"
```

**Options:**
- `-a, --artifact` – Artifact name pattern to download
- `-d, --dir` – Directory to download artifacts to

### run list

List recent runs

```bash
tc run list
tc run list --job Sandbox_Demo
tc run list --status failure --limit 10
tc run list --project Sandbox --branch main
tc run list --since 24h
tc run list --since 24h --until 12h
tc run list --since "2026-01-21"
tc run list --plain | grep failure
```

**Options:**
- `-b, --branch` – Filter by branch name
- `-j, --job` – Filter by job ID
- `--json` – Output as JSON
- `-n, --limit` – Maximum number of runs
- `--no-header` – Omit header row (use with --plain)
- `--plain` – Output in plain text format for scripting
- `-p, --project` – Filter by project ID
- `--since` – Filter builds finished after this time (e.g., 24h, 2026-01-21)
- `--status` – Filter by status (success, failure, running)
- `--until` – Filter builds finished before this time (e.g., 12h, 2026-01-22)
- `-u, --user` – Filter by user who triggered
- `-w, --web` – Open in browser

### run log

View the log output from a run.

```bash
tc run log 12345
tc run log 12345 --failed
tc run log --job Sandbox_Demo
```

**Options:**
- `--failed` – Show only failed step logs
- `-j, --job` – Get log for latest run of this job
- `--raw` – Show raw log without formatting

**Log viewer features:**
- **Mouse/Touchpad scrolling** – Scroll naturally with your trackpad
- **Search** – Press `/` to search forward, `?` to search backward
- **Navigation** – `n`/`N` for next/previous match, `g`/`G` for top/bottom
- **Filter** – `&pattern` to show only matching lines
- **Quit** – Press `q` to exit

Use `--raw` to bypass the pager.

### run pin

Pin a run to prevent it from being automatically cleaned up by retention policies.

```bash
tc run pin 12345
tc run pin 12345 --comment "Release candidate"
```

**Options:**
- `-m, --comment` – Comment explaining why the run is pinned

### run restart

Restart a run with the same configuration.

```bash
tc run restart 12345
tc run restart 12345 --watch
```

**Options:**
- `--watch` – Watch the new run after restarting
- `-w, --web` – Open run in browser

### run start

Start a new run

```bash
tc run start Sandbox_Demo
tc run start Sandbox_Demo --branch feature/test
tc run start Sandbox_Demo -P version=1.0 -S build.number=123 -E CI=true
tc run start Sandbox_Demo --comment "Release build" --tag release --tag v1.0
tc run start Sandbox_Demo --clean --rebuild-deps --top
tc run start Sandbox_Demo --dry-run
```

**Options:**
- `--agent` – Run on specific agent (by ID)
- `-b, --branch` – Branch to build
- `--clean` – Clean sources before run
- `-m, --comment` – Run comment
- `-n, --dry-run` – Show what would be triggered without running
- `-E, --env` – Environment variables (key=value)
- `--json` – Output as JSON (for scripting)
- `-P, --param` – Build parameters (key=value)
- `--personal` – Run as personal build
- `--rebuild-deps` – Rebuild all dependencies
- `--rebuild-failed-deps` – Rebuild failed/incomplete dependencies
- `-S, --system` – System properties (key=value)
- `-t, --tag` – Run tags (can be repeated)
- `--top` – Add to top of queue
- `--watch` – Watch run until it completes
- `-w, --web` – Open run in browser

### run tag

Add one or more tags to a run for categorization and filtering.

```bash
tc run tag 12345 release
tc run tag 12345 release v1.0 production
```

### run tests

Show test results from a run.

```bash
tc run tests 12345
tc run tests 12345 --failed
tc run tests --job Sandbox_Demo
```

**Options:**
- `--failed` – Show only failed tests
- `-j, --job` – Get tests for latest run of this job
- `--json` – Output as JSON
- `-n, --limit` – Maximum number of tests to show

### run unpin

Remove the pin from a run, allowing it to be cleaned up by retention policies.

```bash
tc run unpin 12345
```

### run untag

Remove one or more tags from a run.

```bash
tc run untag 12345 release
tc run untag 12345 release v1.0
```

### run view

View run details

```bash
tc run view 12345
tc run view 12345 --web
tc run view 12345 --json
```

**Options:**
- `--json` – Output as JSON
- `-w, --web` – Open in browser

### run watch

Watch a run in real-time until it completes.

```bash
tc run watch 12345
tc run watch 12345 --interval 10
tc run watch 12345 --logs
```

**Options:**
- `-i, --interval` – Refresh interval in seconds
- `--logs` – Stream build logs while watching

---

## Jobs

### job list

List jobs

```bash
tc job list
tc job list --project Sandbox
tc job list --limit 50
```

**Options:**
- `--json` – Output as JSON
- `-n, --limit` – Maximum number of jobs
- `-p, --project` – Filter by project ID

### job param delete

Delete a parameter from a job.

```bash
tc job param delete MyID MY_PARAM
```

### job param get

Get the value of a specific job parameter.

```bash
tc job param get MyID MY_PARAM
tc job param get MyID VERSION
```

### job param list

List all parameters for a job.

```bash
tc job param list MyID
tc job param list MyID --json
```

**Options:**
- `--json` – Output as JSON

### job param set

Set or update a job parameter value.

```bash
tc job param set MyID MY_PARAM "my value"
tc job param set MyID SECRET_KEY "****" --secure
```

**Options:**
- `--secure` – Mark as secure/password parameter

### job pause

Pause a job to prevent new runs from being triggered.

```bash
tc job pause Sandbox_Demo
```

### job resume

Resume a paused job (build configuration) to allow new runs (builds).

```bash
tc job resume Sandbox_Demo
```

### job view

View job details

```bash
tc job view Sandbox_Demo
tc job view Sandbox_Demo --web
```

**Options:**
- `--json` – Output as JSON
- `-w, --web` – Open in browser

---

## Projects

### project list

List all TeamCity projects.

```bash
tc project list
tc project list --parent AiPlatform
```

**Options:**
- `--json` – Output as JSON
- `-n, --limit` – Maximum number of projects
- `-p, --parent` – Filter by parent project ID

### project param delete

Delete a parameter from a project.

```bash
tc project param delete MyID MY_PARAM
```

### project param get

Get the value of a specific project parameter.

```bash
tc project param get MyID MY_PARAM
tc project param get MyID VERSION
```

### project param list

List all parameters for a project.

```bash
tc project param list MyID
tc project param list MyID --json
```

**Options:**
- `--json` – Output as JSON

### project param set

Set or update a project parameter value.

```bash
tc project param set MyID MY_PARAM "my value"
tc project param set MyID SECRET_KEY "****" --secure
```

**Options:**
- `--secure` – Mark as secure/password parameter

### project token get

Retrieve the original value for a secure token.

```bash
tc project token get Sandbox "credentialsJSON:abc123..."
tc project token get Sandbox "abc123..."
```

### project token put

Store a sensitive value and get a secure token reference.

```bash
# Store a secret interactively (prompts for value)
tc project token put Sandbox

# Store a secret from a value
tc project token put Sandbox "my-secret-password"

# Store a secret from stdin (useful for piping)
echo -n "my-secret" | tc project token put Sandbox --stdin

# Use the token in versioned settings
# password: credentialsJSON:<returned-token>
```

**Options:**
- `--stdin` – Read value from stdin

### project view

View details of a TeamCity project.

```bash
tc project view Sandbox
tc project view Sandbox --web
```

**Options:**
- `--json` – Output as JSON
- `-w, --web` – Open in browser

---

## Queue

### queue approve

Approve a queued run that requires manual approval before it can run.

```bash
tc queue approve 12345
```

### queue list

List all runs in the TeamCity queue.

```bash
tc queue list
tc queue list --job Sandbox_Demo
tc queue list --limit 10
```

**Options:**
- `-j, --job` – Filter by job ID
- `--json` – Output as JSON
- `-n, --limit` – Maximum number of queued runs

### queue remove

Remove a queued run from the TeamCity queue.

```bash
tc queue remove 12345
tc queue remove 12345 --force
```

**Options:**
- `-f, --force` – Skip confirmation prompt

### queue top

Move a queued run to the top of the queue, giving it highest priority.

```bash
tc queue top 12345
```

---

## API

Make an authenticated HTTP request to the TeamCity REST API.

```bash
# Get server info
tc api /app/rest/server

# List projects
tc api /app/rest/projects

# Get a specific build
tc api /app/rest/builds/id:12345

# Create a resource with POST
tc api /app/rest/buildQueue --method POST --field 'buildType=id:MyProject_Build'

# Use custom headers
tc api /app/rest/builds -H "Accept: application/xml"

# Read request body from stdin
echo '{"buildType":{"id":"MyBuild"}}' | tc api /app/rest/buildQueue -X POST --input -

# Include response headers in output
tc api /app/rest/server --include

# Silent mode (only show errors)
tc api /app/rest/server --silent
```

**Options:**
- `-f, --field` – Add a body field as key=value (builds JSON object)
- `-H, --header` – Add a custom header (can be repeated)
- `-i, --include` – Include response headers in output
- `--input` – Read request body from file (use - for stdin)
- `-X, --method` – HTTP method to use
- `--raw` – Output raw response without formatting
- `--silent` – Suppress output on success

<!-- COMMANDS_END -->

## License

Apache-2.0
