# TeamCity CLI
![](https://camo.githubusercontent.com/078d7efd31e09afaa403fc886eac57d43ece79ad24fb75be8e05ac2b13175bef/68747470733a2f2f6a622e67672f6261646765732f6f6666696369616c2d706c61737469632e737667)

A CLI for [TeamCity](https://www.jetbrains.com/teamcity/). Start builds, tail logs, manage agents and queues — without leaving your terminal.

![cli](https://github.com/user-attachments/assets/fa6546f2-5630-4116-aa6c-5addc8d83318)

<details>
<summary> TABLE OF CONTENTS </summary>


<!-- TOC -->
* [TeamCity CLI](#teamcity-cli)
  * [Why tc?](#why-tc)
  * [For AI Agents](#for-ai-agents)
  * [Installation](#installation)
    * [macOS & Linux](#macos--linux)
    * [Windows](#windows)
    * [Go](#go)
    * [Build from source](#build-from-source)
  * [Quick Start](#quick-start)
  * [Commands](#commands)
  * [Configuration](#configuration)
    * [Multiple Servers](#multiple-servers)
  * [Shell Completion](#shell-completion)
  * [Global Flags](#global-flags)
  * [JSON Output](#json-output)
  * [Authentication](#authentication)
    * [auth login](#auth-login)
    * [auth logout](#auth-logout)
    * [auth status](#auth-status)
  * [Runs](#runs)
    * [run artifacts](#run-artifacts)
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
    * [project settings export](#project-settings-export)
    * [project settings status](#project-settings-status)
    * [project settings validate](#project-settings-validate)
    * [project token get](#project-token-get)
    * [project token put](#project-token-put)
    * [project view](#project-view)
  * [Queues](#queues)
    * [queue approve](#queue-approve)
    * [queue list](#queue-list)
    * [queue remove](#queue-remove)
    * [queue top](#queue-top)
  * [Agents](#agents)
    * [agent authorize](#agent-authorize)
    * [agent deauthorize](#agent-deauthorize)
    * [agent disable](#agent-disable)
    * [agent enable](#agent-enable)
    * [agent exec](#agent-exec)
    * [agent jobs](#agent-jobs)
    * [agent list](#agent-list)
    * [agent move](#agent-move)
    * [agent reboot](#agent-reboot)
    * [agent term](#agent-term)
    * [agent view](#agent-view)
  * [Agent Pools](#agent-pools)
    * [pool link](#pool-link)
    * [pool list](#pool-list)
    * [pool unlink](#pool-unlink)
    * [pool view](#pool-view)
  * [API](#api)
  * [Skills](#skills)
    * [skill install](#skill-install)
    * [skill remove](#skill-remove)
    * [skill update](#skill-update)
  * [Contributing](#contributing)
  * [License](#license)
<!-- TOC -->

</details>

## Why tc?

- **[Stay in your terminal](#quick-start)** – Start builds, view logs, manage queues — no browser needed
- **[Remote agent access](#agent-term)** – Shell into any build agent with [`tc agent term`](#agent-term), or run commands with [`tc agent exec`](#agent-exec)
- **[Real-time logs](#run-watch)** – Stream build output as it happens with [`tc run watch --logs`](#run-watch)
- **[Scriptable](#json-output)** – `--json` and `--plain` output for pipelines, plus direct REST API access via [`tc api`](#api)

## For AI Agents

An [Agent Skill](https://agentskills.io) is available for AI coding assistants. It teaches agents how to use `tc` for common TeamCity workflows.

**Claude Code:**
```bash
/plugin marketplace add JetBrains/teamcity-cli
/plugin install teamcity-cli@teamcity-cli
```

The skill is located in [`skills/teamcity-cli/`](skills/teamcity-cli/) and follows the [Agent Skills specification](https://agentskills.io/specification).

## Installation

### macOS & Linux

**Homebrew (recommended):**
```bash
brew install jetbrains/utils/tc
```

**Install script:**
```bash
curl -fsSL https://jb.gg/tc/install | bash
```

**Debian/Ubuntu:**
```bash
curl -fsSLO https://github.com/JetBrains/teamcity-cli/releases/download/v0.4.0/tc_0.4.0_linux_amd64.deb
sudo dpkg -i tc_0.4.0_linux_amd64.deb
```

**RHEL/Fedora:**
```bash
sudo rpm -i https://github.com/JetBrains/teamcity-cli/releases/download/v0.4.0/tc_0.4.0_linux_amd64.rpm
```

**Arch Linux:**
```bash
curl -fsSLO https://github.com/JetBrains/teamcity-cli/releases/download/v0.4.0/tc_0.4.0_linux_amd64.pkg.tar.zst
sudo pacman -U tc_0.4.0_linux_amd64.pkg.tar.zst
```

### Windows

**Winget (recommended):**
```powershell
winget install JetBrains.tc
```

**PowerShell:**
```powershell
irm https://jb.gg/tc/install.ps1 | iex
```

**CMD:**
```cmd
curl -fsSL https://jb.gg/tc/install.cmd -o install.cmd && install.cmd && del install.cmd
```

**Chocolatey:**
```powershell
choco install tc
```

**Scoop:**
```powershell
scoop bucket add jetbrains https://github.com/JetBrains/scoop-utils
scoop install tc
```

### Go

```bash
go install github.com/JetBrains/teamcity-cli/tc@latest
```

### Build from source

```bash
git clone https://github.com/JetBrains/teamcity-cli.git
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

**Auto-detection from DSL:**

When working in a project with TeamCity Kotlin DSL configuration, the server URL is automatically detected from `.teamcity/pom.xml`. This means you can run commands without specifying the server – just ensure you've authenticated with that server previously.

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

## JSON Output

Commands that list resources (`run list`, `job list`, `project list`, `queue list`) support a `--json` flag with field selection:

```bash
# Default fields (default selection covering most use cases)
tc run list --json

# List available fields for a command
tc run list --json=

# Select specific fields
tc run list --json=id,status,webUrl

# Use dot notation for nested fields
tc run list --json=id,status,buildType.name,triggered.user.username
```

**Field notation:**

Use dot notation to access nested fields. For example, `buildType.name` retrieves the build configuration name, and `triggered.user.username` gets the username of who triggered the build.

**Available fields by command:**

| Command        | Example fields                                                                                                                                                                                        |
|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `run list`     | `id`, `number`, `status`, `state`, `branchName`, `buildTypeId`, `buildType.name`, `buildType.projectName`, `triggered.type`, `triggered.user.name`, `agent.name`, `startDate`, `finishDate`, `webUrl` |
| `job list`     | `id`, `name`, `projectName`, `projectId`, `paused`, `href`, `webUrl`                                                                                                                                  |
| `project list` | `id`, `name`, `description`, `parentProjectId`, `href`, `webUrl`                                                                                                                                      |
| `queue list`   | `id`, `buildTypeId`, `state`, `branchName`, `queuedDate`, `buildType.name`, `triggered.user.name`, `webUrl`                                                                                           |

Run `tc <command> --json=` to see all available fields for that command.

**Scripting examples:**

```bash
# Get build IDs that failed
tc run list --status failure --json=id | jq -r '.[].id'

# Export runs to CSV
tc run list --json=id,status,branchName | jq -r '.[] | [.id,.status,.branchName] | @csv'

# Get web URLs for queued builds
tc queue list --json=webUrl | jq -r '.[].webUrl'
```

<!-- COMMANDS_START -->

## Authentication

### auth login

Authenticate with a TeamCity server using an access token.

This will:
1. Prompt for your TeamCity server URL
2. Open your browser to generate an access token
3. Validate and store the token securely

For CI/CD, use environment variables instead:
  export TEAMCITY_URL="https://teamcity.example.com"
  export TEAMCITY_TOKEN="your-access-token"

When running inside a TeamCity build, authentication is automatic using
build-level credentials from the build properties file.

**Options:**
- `-s, --server` – TeamCity server URL
- `-t, --token` – Access token

### auth logout

Log out from a TeamCity server

### auth status

Show authentication status

---

## Runs

### run artifacts

List artifacts from a run without downloading them.

Shows artifact names and sizes. Use tc run download to download artifacts.

```bash
tc run artifacts 12345
tc run artifacts 12345 --json
tc run artifacts --job MyBuild
```

**Options:**
- `-j, --job` – List artifacts from latest run of this job
- `--json` – Output as JSON

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

Without a comment argument, displays the current comment.
With a comment argument, sets the comment.
Use --delete to remove the comment.

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
tc run list --job Falcon_Build
tc run list --status failure --limit 10
tc run list --project Falcon --branch main
tc run list --since 24h
tc run list --json
tc run list --json=id,status,webUrl
tc run list --plain | grep failure
```

**Options:**
- `-b, --branch` – Filter by branch name
- `-j, --job` – Filter by job ID
- `--json` – Output JSON with fields (use --json= to list, --json=f1,f2 for specific)
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

You can specify a run ID directly, or use --job to get the latest run's log.

Pager: / search, n/N next/prev, g/G top/bottom, q quit.
Use --raw to bypass the pager.

```bash
tc run log 12345
tc run log 12345 --failed
tc run log --job Falcon_Build
```

**Options:**
- `--failed` – Show failure summary (problems and failed tests)
- `-j, --job` – Get log for latest run of this job
- `--raw` – Show raw log without formatting

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
tc run start Falcon_Build
tc run start Falcon_Build --branch feature/test
tc run start Falcon_Build -P version=1.0 -S build.number=123 -E CI=true
tc run start Falcon_Build --comment "Release build" --tag release --tag v1.0
tc run start Falcon_Build --clean --rebuild-deps --top
tc run start Falcon_Build --local-changes # personal build with uncommitted Git changes
tc run start Falcon_Build --local-changes changes.patch  # from file
tc run start Falcon_Build --revision abc123def --branch main
tc run start Falcon_Build --dry-run
```

**Options:**
- `--agent` – Run on specific agent (by ID)
- `-b, --branch` – Branch to build
- `--clean` – Clean sources before run
- `-m, --comment` – Run comment
- `-n, --dry-run` – Show what would be triggered without running
- `-E, --env` – Environment variables (key=value)
- `--json` – Output as JSON (for scripting)
- `-l, --local-changes` – Include local changes (git, -, or path; default: git)
- `--no-push` – Skip auto-push of branch to remote
- `-P, --param` – Build parameters (key=value)
- `--personal` – Run as personal build
- `--rebuild-deps` – Rebuild all dependencies
- `--rebuild-failed-deps` – Rebuild failed/incomplete dependencies
- `--revision` – Pin build to a specific Git commit SHA
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

You can specify a run ID directly, or use --job to get the latest run's tests.

```bash
tc run tests 12345
tc run tests 12345 --failed
tc run tests --job Falcon_Build
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
- `-Q, --quiet` – Minimal output, show only state changes and result
- `--timeout` – Timeout duration (e.g., 30m, 1h)

---

## Jobs

### job list

List jobs

```bash
tc job list
tc job list --project Falcon
tc job list --json
tc job list --json=id,name,webUrl
```

**Options:**
- `--json` – Output JSON with fields (use --json= to list, --json=f1,f2 for specific)
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
tc job pause Falcon_Build
```

### job resume

Resume a paused job to allow new runs.

```bash
tc job resume Falcon_Build
```

### job view

View job details

```bash
tc job view Falcon_Build
tc job view Falcon_Build --web
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
tc project list --parent Falcon
tc project list --json
tc project list --json=id,name,webUrl
```

**Options:**
- `--json` – Output JSON with fields (use --json= to list, --json=f1,f2 for specific)
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

### project settings export

Export project settings as a ZIP archive containing Kotlin DSL or XML configuration.

The exported archive can be used to:
- Version control your CI/CD configuration
- Migrate settings between TeamCity instances
- Review settings as code

By default, exports in Kotlin DSL format.

```bash
# Export as Kotlin DSL (default)
tc project settings export MyProject

# Export as Kotlin DSL explicitly
tc project settings export MyProject --kotlin

# Export as XML
tc project settings export MyProject --xml

# Save to specific file
tc project settings export MyProject -o settings.zip
```

**Options:**
- `--kotlin` – Export as Kotlin DSL (default)
- `-o, --output` – Output file path (default: projectSettings.zip)
- `--relative-ids` – Use relative IDs in exported settings
- `--xml` – Export as XML

### project settings status

Show the synchronization status of versioned settings for a project.

Displays:
- Whether versioned settings are enabled
- Current sync state (up-to-date, pending changes, errors)
- Last successful sync timestamp
- VCS root and format information
- Any warnings or errors from the last sync attempt

```bash
tc project settings status MyProject
tc project settings status MyProject --json
```

**Options:**
- `--json` – Output as JSON

### project settings validate

Validate Kotlin DSL configuration by running mvn teamcity-configs:generate.

Auto-detects .teamcity directory in the current directory or parents.
Requires Maven (mvn) or uses mvnw wrapper if present in the DSL directory.

```bash
tc project settings validate
tc project settings validate ./path/to/.teamcity
tc project settings validate --verbose
```

**Options:**
- `-v, --verbose` – Show full Maven output

### project token get

Retrieve the original value for a secure token.

This operation requires CHANGE_SERVER_SETTINGS permission,
which is only available to System Administrators.

```bash
tc project token get Falcon "credentialsJSON:abc123..."
tc project token get Falcon "abc123..."
```

### project token put

Store a sensitive value and get a secure token reference.

The returned token can be used in versioned settings configuration files
as credentialsJSON:<token>. The actual value is stored securely in TeamCity
and is not committed to version control.

Requires EDIT_PROJECT permission (Project Administrator role).

```bash
# Store a secret interactively (prompts for value)
tc project token put Falcon

# Store a secret from a value
tc project token put Falcon "my-secret-password"

# Store a secret from stdin (useful for piping)
echo -n "my-secret" | tc project token put Falcon --stdin

# Use the token in versioned settings
# password: credentialsJSON:<returned-token>
```

**Options:**
- `--stdin` – Read value from stdin

### project view

View details of a TeamCity project.

```bash
tc project view Falcon
tc project view Falcon --web
```

**Options:**
- `--json` – Output as JSON
- `-w, --web` – Open in browser

---

## Queues

### queue approve

Approve a queued run that requires manual approval before it can run.

```bash
tc queue approve 12345
```

### queue list

List all runs in the TeamCity queue.

```bash
tc queue list
tc queue list --job Falcon_Build
tc queue list --json
tc queue list --json=id,state,webUrl
```

**Options:**
- `-j, --job` – Filter by job ID
- `--json` – Output JSON with fields (use --json= to list, --json=f1,f2 for specific)
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

## Agents

### agent authorize

Authorize an agent to allow it to connect and run builds.

```bash
tc agent authorize 1
tc agent authorize Agent-Linux-01
```

### agent deauthorize

Deauthorize an agent to revoke its permission to connect.

```bash
tc agent deauthorize 1
tc agent deauthorize Agent-Linux-01
```

### agent disable

Disable an agent to prevent it from running builds.

```bash
tc agent disable 1
tc agent disable Agent-Linux-01
```

### agent enable

Enable an agent to allow it to run builds.

```bash
tc agent enable 1
tc agent enable Agent-Linux-01
```

### agent exec

Execute a command on a TeamCity build agent and return the output.

```bash
tc agent exec 1 "ls -la"
tc agent exec Agent-Linux-01 "cat /etc/os-release"
tc agent exec Agent-Linux-01 --timeout 10m -- long-running-script.sh
```

**Options:**
- `--timeout` – Command timeout

### agent jobs

List build configurations (jobs) that are compatible or incompatible with an agent.

```bash
tc agent jobs 1
tc agent jobs Agent-Linux-01
tc agent jobs Agent-Linux-01 --incompatible
tc agent jobs 1 --json
```

**Options:**
- `--incompatible` – Show incompatible jobs with reasons
- `--json` – Output as JSON

### agent list

List build agents

```bash
tc agent list
tc agent list --pool Default
tc agent list --connected
tc agent list --json
tc agent list --json=id,name,connected,enabled
```

**Options:**
- `--authorized` – Show only authorized agents
- `--connected` – Show only connected agents
- `--enabled` – Show only enabled agents
- `--json` – Output JSON with fields (use --json= to list, --json=f1,f2 for specific)
- `-n, --limit` – Maximum number of agents
- `-p, --pool` – Filter by agent pool

### agent move

Move an agent to a different agent pool.

```bash
tc agent move 1 0
tc agent move Agent-Linux-01 2
```

### agent reboot

Request a reboot of a build agent.

The agent can be specified by ID or name. By default, the agent reboots immediately.
Use --after-build to wait for the current build to finish before rebooting.

Note: Local agents (running on the same machine as the server) cannot be rebooted.

```bash
tc agent reboot 1
tc agent reboot Agent-Linux-01
tc agent reboot Agent-Linux-01 --after-build
tc agent reboot Agent-Linux-01 --yes
```

**Options:**
- `--after-build` – Wait for current build to finish before rebooting
- `-y, --yes` – Skip confirmation prompt

### agent term

Open an interactive shell session to a TeamCity build agent.

```bash
tc agent term 1
tc agent term Agent-Linux-01
```

### agent view

View agent details

```bash
tc agent view 1
tc agent view Agent-Linux-01
tc agent view Agent-Linux-01 --web
tc agent view 1 --json
```

**Options:**
- `--json` – Output as JSON
- `-w, --web` – Open in browser

---

## Agent Pools

### pool link

Link a project to an agent pool, allowing the project's builds to run on agents in that pool.

```bash
tc pool link 1 MyProject
```

### pool list

List agent pools

```bash
tc pool list
tc pool list --json
tc pool list --json=id,name,maxAgents
```

**Options:**
- `--json` – Output JSON with fields (use --json= to list, --json=f1,f2 for specific)

### pool unlink

Unlink a project from an agent pool, removing the project's access to agents in that pool.

```bash
tc pool unlink 1 MyProject
```

### pool view

View pool details

```bash
tc pool view 0
tc pool view 1 --web
tc pool view 1 --json
```

**Options:**
- `--json` – Output as JSON
- `-w, --web` – Open in browser

---

## API

Make an authenticated HTTP request to the TeamCity REST API.

The endpoint argument should be the path portion of the URL,
starting with /app/rest/. The base URL and authentication
are handled automatically.

This command is useful for:
- Accessing API features not yet supported by the CLI
- Scripting and automation
- Debugging and exploration

```bash
# Get server info
tc api /app/rest/server

# List projects
tc api /app/rest/projects

# Create a resource with POST
tc api /app/rest/buildQueue -X POST -f 'buildType=id:MyBuild'

# Fetch all pages and combine into array
tc api /app/rest/builds --paginate --slurp
```

**Options:**
- `-f, --field` – Add a body field as key=value (builds JSON object)
- `-H, --header` – Add a custom header (can be repeated)
- `-i, --include` – Include response headers in output
- `--input` – Read request body from file (use - for stdin)
- `-X, --method` – HTTP method to use
- `--paginate` – Make additional requests to fetch all pages
- `--raw` – Output raw response without formatting
- `--silent` – Suppress output on success
- `--slurp` – Combine paginated results into a JSON array (requires --paginate)

---

## Skills

### skill install

Install the teamcity-cli skill so AI coding agents can use tc commands.

Installs globally by default. Use --project to install to the current project only.
Auto-detects installed agents when --agent is not specified.

```bash
tc skill install
tc skill install --agent claude-code --agent cursor
tc skill install --project
```

**Options:**
- `-a, --agent` – Target agent(s); auto-detects if omitted
- `--project` – Install to current project instead of globally

### skill remove

Remove the teamcity-cli skill from AI coding agents

```bash
tc skill remove
tc skill remove --agent claude-code
tc skill remove --project
```

**Options:**
- `-a, --agent` – Target agent(s); auto-detects if omitted
- `--project` – Install to current project instead of globally

### skill update

Update the teamcity-cli skill to the latest version bundled with this tc release.

Skips if the installed version already matches.
Auto-detects installed agents when --agent is not specified.

```bash
tc skill update
tc skill update --agent claude-code
tc skill update --project
```

**Options:**
- `-a, --agent` – Target agent(s); auto-detects if omitted
- `--project` – Install to current project instead of globally

<!-- COMMANDS_END -->

## Contributing

Want to help? See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and guidelines.

## License

Apache-2.0
