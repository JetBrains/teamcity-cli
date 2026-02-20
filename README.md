# TeamCity CLI
![](https://camo.githubusercontent.com/078d7efd31e09afaa403fc886eac57d43ece79ad24fb75be8e05ac2b13175bef/68747470733a2f2f6a622e67672f6261646765732f6f6666696369616c2d706c61737469632e737667)

A CLI for [TeamCity](https://www.jetbrains.com/teamcity/). Start builds, tail logs, manage agents and queues — without leaving your terminal.

![cli](https://github.com/user-attachments/assets/fa6546f2-5630-4116-aa6c-5addc8d83318)

<details>
<summary> TABLE OF CONTENTS </summary>


<!-- TOC -->
* [TeamCity CLI](#teamcity-cli)
  * [Why teamcity?](#why-teamcity)
  * [Installation](#installation)
    * [macOS & Linux](#macos--linux)
    * [Windows](#windows)
    * [Go](#go)
    * [Build from source](#build-from-source)
  * [Quick Start](#quick-start)
  * [For AI Agents](#for-ai-agents)
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
  * [Aliases](#aliases)
    * [alias delete](#alias-delete)
    * [alias list](#alias-list)
    * [alias set](#alias-set)
  * [Skills](#skills)
    * [skill install](#skill-install)
    * [skill remove](#skill-remove)
    * [skill update](#skill-update)
  * [Awesome Aliases](#awesome-aliases)
  * [Contributing](#contributing)
  * [License](#license)
<!-- TOC -->

</details>

## Why teamcity?

- **[Stay in your terminal](#quick-start)** – Start builds, view logs, manage queues — no browser needed
- **[Remote agent access](#agent-term)** – Shell into any build agent with [`teamcity agent term`](#agent-term), or run commands with [`teamcity agent exec`](#agent-exec)
- **[Real-time logs](#run-watch)** – Stream build output as it happens with [`teamcity run watch --logs`](#run-watch)
- **[Scriptable](#json-output)** – `--json` and `--plain` output for pipelines, plus direct REST API access via [`teamcity api`](#api)
- **[AI agent ready](#for-ai-agents)** – Built-in [skill](https://agentskills.io) for Claude Code, Cursor, and other AI coding agents — just run `teamcity skill install`

## Installation

### macOS & Linux

**Homebrew (recommended):**
```bash
brew install jetbrains/utils/teamcity
```

**Install script:**
```bash
curl -fsSL https://jb.gg/tc/install | bash
```

**Debian/Ubuntu:**
```bash
curl -fsSLO https://github.com/JetBrains/teamcity-cli/releases/latest/download/teamcity_linux_amd64.deb
sudo dpkg -i teamcity_linux_amd64.deb
```

**RHEL/Fedora:**
```bash
sudo rpm -i https://github.com/JetBrains/teamcity-cli/releases/latest/download/teamcity_linux_amd64.rpm
```

**Arch Linux:**
```bash
curl -fsSLO https://github.com/JetBrains/teamcity-cli/releases/latest/download/teamcity_linux_amd64.pkg.tar.zst
sudo pacman -U teamcity_linux_amd64.pkg.tar.zst
```

### Windows

**Winget (recommended):**
```powershell
winget install JetBrains.TeamCityCLI
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
choco install TeamCityCLI
```

**Scoop:**
```powershell
scoop bucket add jetbrains https://github.com/JetBrains/scoop-utils
scoop install teamcity
```

### Go

```bash
go install github.com/JetBrains/teamcity-cli/tc@latest
```

### Build from source

```bash
git clone https://github.com/JetBrains/teamcity-cli.git
cd teamcity-cli
go build -o teamcity ./tc
```

## Quick Start

```bash
# Authenticate with your TeamCity server
teamcity auth login

# List recent builds
teamcity run list --limit 10

# Start a build and watch it run
teamcity run start MyProject_Build --branch main --watch

# View logs from the latest build of a job
teamcity run log --job MyProject_Build

# Check what's in the queue
teamcity queue list
```

## For AI Agents

An [Agent Skill](https://agentskills.io) is included with `teamcity`. It teaches AI coding agents how to use `teamcity` for common TeamCity workflows.

```bash
teamcity skill install           # auto-detects installed agents (Claude Code, Cursor, etc.)
teamcity skill install --project # install to current project only
teamcity skill update            # update to latest version bundled with teamcity
teamcity skill remove            # uninstall
```

or specifically for **Claude Code:**
```bash
/plugin marketplace add JetBrains/teamcity-cli
/plugin install teamcity-cli@teamcity-cli
```

The skill is located in [`skills/teamcity-cli/`](skills/teamcity-cli/) and follows the [Agent Skills specification](https://agentskills.io/specification).

## Commands

[**auth**](#authentication) · [login](#auth-login) · [logout](#auth-logout) · [status](#auth-status)

[**run**](#runs) · [list](#run-list) · [start](#run-start) · [view](#run-view) · [watch](#run-watch) · [log](#run-log) · [changes](#run-changes) · [tests](#run-tests) · [cancel](#run-cancel) · [download](#run-download) · [artifacts](#run-artifacts) · [restart](#run-restart) · [pin](#run-pin)/[unpin](#run-unpin) · [tag](#run-tag)/[untag](#run-untag) · [comment](#run-comment)

[**job**](#jobs) · [list](#job-list) · [view](#job-view) · [pause](#job-pause)/[resume](#job-resume) · [param](#job-param-list)

[**project**](#projects) · [list](#project-list) · [view](#project-view) · [param](#project-param-list) · [token](#project-token-get) · [settings](#project-settings-export)

[**queue**](#queues) · [list](#queue-list) · [approve](#queue-approve) · [remove](#queue-remove) · [top](#queue-top)

[**agent**](#agents) · [list](#agent-list) · [view](#agent-view) · [term](#agent-term) · [exec](#agent-exec) · [jobs](#agent-jobs) · [authorize](#agent-authorize)/[deauthorize](#agent-deauthorize) · [enable](#agent-enable)/[disable](#agent-disable) · [move](#agent-move) · [reboot](#agent-reboot)

[**pool**](#agent-pools) · [list](#pool-list) · [view](#pool-view) · [link](#pool-link)/[unlink](#pool-unlink)

[**api**](#api)

[**alias**](#aliases) · [set](#alias-set) · [list](#alias-list) · [delete](#alias-delete)

[**skill**](#skills) · [install](#skill-install) · [remove](#skill-remove) · [update](#skill-update)

## Configuration

Tokens are stored in your system keyring (macOS Keychain, GNOME Keyring, Windows Credential Manager) when available. The config file at `~/.config/tc/config.yml` stores server URLs and usernames:

```yaml
default_server: https://teamcity.example.com
servers:
  https://teamcity.example.com:
    user: username
```

If the system keyring is unavailable (or `--insecure-storage` is used), the token is stored in the config file instead.

### Multiple Servers

You can authenticate with multiple TeamCity servers. Each server's credentials are stored separately.

**Adding servers:**

```bash
# Log in to your first server
teamcity auth login --server https://teamcity1.example.com

# Log in to additional servers (becomes the new default)
teamcity auth login --server https://teamcity2.example.com
```

**Switching between servers:**

```bash
# Option 1: Use environment variable (recommended for scripts)
TEAMCITY_URL=https://teamcity1.example.com teamcity run list

# Option 2: Export for your session
export TEAMCITY_URL=https://teamcity1.example.com
teamcity run list  # uses teamcity1
teamcity auth status  # shows teamcity1

# Option 3: Log in again to change the default
teamcity auth login --server https://teamcity1.example.com
```

**Example multi-server config:**

```yaml
default_server: https://teamcity-prod.example.com
servers:
  https://teamcity-prod.example.com:
    user: alice
  https://teamcity-staging.example.com:
    user: alice
  https://teamcity-dev.example.com:
    user: alice
```

**CI/CD usage:**

Environment variables always take precedence over config file settings:

```bash
export TEAMCITY_URL="https://teamcity.example.com"
export TEAMCITY_TOKEN="your-access-token"
teamcity run start MyProject_Build  # uses env vars, ignores config file
```

**Auto-detection from DSL:**

When working in a project with TeamCity Kotlin DSL configuration, the server URL is automatically detected from `.teamcity/pom.xml`. This means you can run commands without specifying the server – just ensure you've authenticated with that server previously.

## Shell Completion

```bash
# Bash
teamcity completion bash > /etc/bash_completion.d/teamcity

# Zsh
teamcity completion zsh > "${fpath[1]}/_teamcity"

# Fish
teamcity completion fish > ~/.config/fish/completions/teamcity.fish

# PowerShell
teamcity completion powershell > teamcity.ps1
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
teamcity run list --json

# List available fields for a command
teamcity run list --json=

# Select specific fields
teamcity run list --json=id,status,webUrl

# Use dot notation for nested fields
teamcity run list --json=id,status,buildType.name,triggered.user.username
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

Run `teamcity <command> --json=` to see all available fields for that command.

**Scripting examples:**

```bash
# Get build IDs that failed
teamcity run list --status failure --json=id | jq -r '.[].id'

# Export runs to CSV
teamcity run list --json=id,status,branchName | jq -r '.[] | [.id,.status,.branchName] | @csv'

# Get web URLs for queued builds
teamcity queue list --json=webUrl | jq -r '.[].webUrl'
```

<details>
<summary><b>Command Reference</b> (click to expand)</summary>

<!-- COMMANDS_START -->

## Authentication

### auth login

Authenticate with a TeamCity server using an access token.

This will:
1. Prompt for your TeamCity server URL
2. Open your browser to generate an access token
3. Validate and store the token securely

The token is stored in your system keyring (macOS Keychain, GNOME Keyring,
Windows Credential Manager) when available. Use --insecure-storage to store
the token in plain text in the config file instead.

For guest access (read-only, no token needed; must be enabled on the server):
  teamcity auth login -s https://teamcity.example.com --guest

For CI/CD, use environment variables instead:
  export TEAMCITY_URL="https://teamcity.example.com"
  export TEAMCITY_TOKEN="your-access-token"
  # Or for guest access:
  export TEAMCITY_URL="https://teamcity.example.com"
  export TEAMCITY_GUEST=1

When running inside a TeamCity build, authentication is automatic using
build-level credentials from the build properties file.

**Options:**
- `--guest` – Use guest authentication (no token needed, must be enabled on the server)
- `--insecure-storage` – Store token in plain text config file instead of system keyring
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

Shows artifact names and sizes. Use teamcity run download to download artifacts.

```bash
teamcity run artifacts 12345
teamcity run artifacts 12345 --json
teamcity run artifacts 12345 --path html_reports/coverage
teamcity run artifacts --job MyBuild
```

**Options:**
- `-j, --job` – List artifacts from latest run of this job
- `--json` – Output as JSON
- `-p, --path` – Browse artifacts under this subdirectory

### run cancel

Cancel a running or queued run.

```bash
teamcity run cancel 12345
teamcity run cancel 12345 --comment "Cancelling for hotfix"
teamcity run cancel 12345 --force
```

**Options:**
- `--comment` – Comment for cancellation
- `-f, --force` – Skip confirmation prompt

### run changes

Show the VCS changes (commits) included in a run.

```bash
teamcity run changes 12345
teamcity run changes 12345 --no-files
teamcity run changes 12345 --json
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
teamcity run comment 12345
teamcity run comment 12345 "Deployed to production"
teamcity run comment 12345 --delete
```

**Options:**
- `--delete` – Delete the comment

### run download

Download artifacts from a completed run.

```bash
teamcity run download 12345
teamcity run download 12345 --dir ./artifacts
teamcity run download 12345 --artifact "*.jar"
```

**Options:**
- `-a, --artifact` – Artifact name pattern to download
- `-d, --dir` – Directory to download artifacts to

### run list

List recent runs

```bash
teamcity run list
teamcity run list --job Falcon_Build
teamcity run list --status failure --limit 10
teamcity run list --project Falcon --branch main
teamcity run list --since 24h
teamcity run list --json
teamcity run list --json=id,status,webUrl
teamcity run list --plain | grep failure
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
teamcity run log 12345
teamcity run log 12345 --failed
teamcity run log --job Falcon_Build
```

**Options:**
- `--failed` – Show failure summary (problems and failed tests)
- `-j, --job` – Get log for latest run of this job
- `--raw` – Show raw log without formatting

### run pin

Pin a run to prevent it from being automatically cleaned up by retention policies.

```bash
teamcity run pin 12345
teamcity run pin 12345 --comment "Release candidate"
```

**Options:**
- `-m, --comment` – Comment explaining why the run is pinned

### run restart

Restart a run with the same configuration.

```bash
teamcity run restart 12345
teamcity run restart 12345 --watch
```

**Options:**
- `--watch` – Watch the new run after restarting
- `-w, --web` – Open run in browser

### run start

Start a new run

```bash
teamcity run start Falcon_Build
teamcity run start Falcon_Build --branch feature/test
teamcity run start Falcon_Build -P version=1.0 -S build.number=123 -E CI=true
teamcity run start Falcon_Build --comment "Release build" --tag release --tag v1.0
teamcity run start Falcon_Build --clean --rebuild-deps --top
teamcity run start Falcon_Build --local-changes # personal build with uncommitted Git changes
teamcity run start Falcon_Build --local-changes changes.patch  # from file
teamcity run start Falcon_Build --revision abc123def --branch main
teamcity run start Falcon_Build --dry-run
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
teamcity run tag 12345 release
teamcity run tag 12345 release v1.0 production
```

### run tests

Show test results from a run.

You can specify a run ID directly, or use --job to get the latest run's tests.

```bash
teamcity run tests 12345
teamcity run tests 12345 --failed
teamcity run tests --job Falcon_Build
```

**Options:**
- `--failed` – Show only failed tests
- `-j, --job` – Get tests for latest run of this job
- `--json` – Output as JSON
- `-n, --limit` – Maximum number of tests to show

### run unpin

Remove the pin from a run, allowing it to be cleaned up by retention policies.

```bash
teamcity run unpin 12345
```

### run untag

Remove one or more tags from a run.

```bash
teamcity run untag 12345 release
teamcity run untag 12345 release v1.0
```

### run view

View run details

```bash
teamcity run view 12345
teamcity run view 12345 --web
teamcity run view 12345 --json
```

**Options:**
- `--json` – Output as JSON
- `-w, --web` – Open in browser

### run watch

Watch a run in real-time until it completes.

```bash
teamcity run watch 12345
teamcity run watch 12345 --interval 10
teamcity run watch 12345 --logs
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
teamcity job list
teamcity job list --project Falcon
teamcity job list --json
teamcity job list --json=id,name,webUrl
```

**Options:**
- `--json` – Output JSON with fields (use --json= to list, --json=f1,f2 for specific)
- `-n, --limit` – Maximum number of jobs
- `-p, --project` – Filter by project ID

### job param delete

Delete a parameter from a job.

```bash
teamcity job param delete MyID MY_PARAM
```

### job param get

Get the value of a specific job parameter.

```bash
teamcity job param get MyID MY_PARAM
teamcity job param get MyID VERSION
```

### job param list

List all parameters for a job.

```bash
teamcity job param list MyID
teamcity job param list MyID --json
```

**Options:**
- `--json` – Output as JSON

### job param set

Set or update a job parameter value.

```bash
teamcity job param set MyID MY_PARAM "my value"
teamcity job param set MyID SECRET_KEY "****" --secure
```

**Options:**
- `--secure` – Mark as secure/password parameter

### job pause

Pause a job to prevent new runs from being triggered.

```bash
teamcity job pause Falcon_Build
```

### job resume

Resume a paused job to allow new runs.

```bash
teamcity job resume Falcon_Build
```

### job view

View job details

```bash
teamcity job view Falcon_Build
teamcity job view Falcon_Build --web
```

**Options:**
- `--json` – Output as JSON
- `-w, --web` – Open in browser

---

## Projects

### project list

List all TeamCity projects.

```bash
teamcity project list
teamcity project list --parent Falcon
teamcity project list --json
teamcity project list --json=id,name,webUrl
```

**Options:**
- `--json` – Output JSON with fields (use --json= to list, --json=f1,f2 for specific)
- `-n, --limit` – Maximum number of projects
- `-p, --parent` – Filter by parent project ID

### project param delete

Delete a parameter from a project.

```bash
teamcity project param delete MyID MY_PARAM
```

### project param get

Get the value of a specific project parameter.

```bash
teamcity project param get MyID MY_PARAM
teamcity project param get MyID VERSION
```

### project param list

List all parameters for a project.

```bash
teamcity project param list MyID
teamcity project param list MyID --json
```

**Options:**
- `--json` – Output as JSON

### project param set

Set or update a project parameter value.

```bash
teamcity project param set MyID MY_PARAM "my value"
teamcity project param set MyID SECRET_KEY "****" --secure
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
teamcity project settings export MyProject

# Export as Kotlin DSL explicitly
teamcity project settings export MyProject --kotlin

# Export as XML
teamcity project settings export MyProject --xml

# Save to specific file
teamcity project settings export MyProject -o settings.zip
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
teamcity project settings status MyProject
teamcity project settings status MyProject --json
```

**Options:**
- `--json` – Output as JSON

### project settings validate

Validate Kotlin DSL configuration by running mvn teamcity-configs:generate.

Auto-detects .teamcity directory in the current directory or parents.
Requires Maven (mvn) or uses mvnw wrapper if present in the DSL directory.

```bash
teamcity project settings validate
teamcity project settings validate ./path/to/.teamcity
teamcity project settings validate --verbose
```

**Options:**
- `-v, --verbose` – Show full Maven output

### project token get

Retrieve the original value for a secure token.

This operation requires CHANGE_SERVER_SETTINGS permission,
which is only available to System Administrators.

```bash
teamcity project token get Falcon "credentialsJSON:abc123..."
teamcity project token get Falcon "abc123..."
```

### project token put

Store a sensitive value and get a secure token reference.

The returned token can be used in versioned settings configuration files
as credentialsJSON:<token>. The actual value is stored securely in TeamCity
and is not committed to version control.

Requires EDIT_PROJECT permission (Project Administrator role).

```bash
# Store a secret interactively (prompts for value)
teamcity project token put Falcon

# Store a secret from a value
teamcity project token put Falcon "my-secret-password"

# Store a secret from stdin (useful for piping)
echo -n "my-secret" | teamcity project token put Falcon --stdin

# Use the token in versioned settings
# password: credentialsJSON:<returned-token>
```

**Options:**
- `--stdin` – Read value from stdin

### project view

View details of a TeamCity project.

```bash
teamcity project view Falcon
teamcity project view Falcon --web
```

**Options:**
- `--json` – Output as JSON
- `-w, --web` – Open in browser

---

## Queues

### queue approve

Approve a queued run that requires manual approval before it can run.

```bash
teamcity queue approve 12345
```

### queue list

List all runs in the TeamCity queue.

```bash
teamcity queue list
teamcity queue list --job Falcon_Build
teamcity queue list --json
teamcity queue list --json=id,state,webUrl
```

**Options:**
- `-j, --job` – Filter by job ID
- `--json` – Output JSON with fields (use --json= to list, --json=f1,f2 for specific)
- `-n, --limit` – Maximum number of queued runs

### queue remove

Remove a queued run from the TeamCity queue.

```bash
teamcity queue remove 12345
teamcity queue remove 12345 --force
```

**Options:**
- `-f, --force` – Skip confirmation prompt

### queue top

Move a queued run to the top of the queue, giving it highest priority.

```bash
teamcity queue top 12345
```

---

## Agents

### agent authorize

Authorize an agent to allow it to connect and run builds.

```bash
teamcity agent authorize 1
teamcity agent authorize Agent-Linux-01
```

### agent deauthorize

Deauthorize an agent to revoke its permission to connect.

```bash
teamcity agent deauthorize 1
teamcity agent deauthorize Agent-Linux-01
```

### agent disable

Disable an agent to prevent it from running builds.

```bash
teamcity agent disable 1
teamcity agent disable Agent-Linux-01
```

### agent enable

Enable an agent to allow it to run builds.

```bash
teamcity agent enable 1
teamcity agent enable Agent-Linux-01
```

### agent exec

Execute a command on a TeamCity build agent and return the output.

```bash
teamcity agent exec 1 "ls -la"
teamcity agent exec Agent-Linux-01 "cat /etc/os-release"
teamcity agent exec Agent-Linux-01 --timeout 10m -- long-running-script.sh
```

**Options:**
- `--timeout` – Command timeout

### agent jobs

List build configurations (jobs) that are compatible or incompatible with an agent.

```bash
teamcity agent jobs 1
teamcity agent jobs Agent-Linux-01
teamcity agent jobs Agent-Linux-01 --incompatible
teamcity agent jobs 1 --json
```

**Options:**
- `--incompatible` – Show incompatible jobs with reasons
- `--json` – Output as JSON

### agent list

List build agents

```bash
teamcity agent list
teamcity agent list --pool Default
teamcity agent list --connected
teamcity agent list --json
teamcity agent list --json=id,name,connected,enabled
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
teamcity agent move 1 0
teamcity agent move Agent-Linux-01 2
```

### agent reboot

Request a reboot of a build agent.

The agent can be specified by ID or name. By default, the agent reboots immediately.
Use --after-build to wait for the current build to finish before rebooting.

Note: Local agents (running on the same machine as the server) cannot be rebooted.

```bash
teamcity agent reboot 1
teamcity agent reboot Agent-Linux-01
teamcity agent reboot Agent-Linux-01 --after-build
teamcity agent reboot Agent-Linux-01 --yes
```

**Options:**
- `--after-build` – Wait for current build to finish before rebooting
- `-y, --yes` – Skip confirmation prompt

### agent term

Open an interactive shell session to a TeamCity build agent.

```bash
teamcity agent term 1
teamcity agent term Agent-Linux-01
```

### agent view

View agent details

```bash
teamcity agent view 1
teamcity agent view Agent-Linux-01
teamcity agent view Agent-Linux-01 --web
teamcity agent view 1 --json
```

**Options:**
- `--json` – Output as JSON
- `-w, --web` – Open in browser

---

## Agent Pools

### pool link

Link a project to an agent pool, allowing the project's builds to run on agents in that pool.

```bash
teamcity pool link 1 MyProject
```

### pool list

List agent pools

```bash
teamcity pool list
teamcity pool list --json
teamcity pool list --json=id,name,maxAgents
```

**Options:**
- `--json` – Output JSON with fields (use --json= to list, --json=f1,f2 for specific)

### pool unlink

Unlink a project from an agent pool, removing the project's access to agents in that pool.

```bash
teamcity pool unlink 1 MyProject
```

### pool view

View pool details

```bash
teamcity pool view 0
teamcity pool view 1 --web
teamcity pool view 1 --json
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
teamcity api /app/rest/server

# List projects
teamcity api /app/rest/projects

# Create a resource with POST
teamcity api /app/rest/buildQueue -X POST -f 'buildType=id:MyBuild'

# Fetch all pages and combine into array
teamcity api /app/rest/builds --paginate --slurp
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

## Aliases

### alias delete

Delete an alias

### alias list

List configured aliases

**Options:**
- `--json` – Output as JSON

### alias set

Create a shortcut that expands into a full teamcity command.

Use $1, $2, ... for positional arguments. Extra arguments are appended.
Use --shell for aliases that need pipes, redirection, or other shell features.

```bash
# Quick shortcuts
teamcity alias set rl  'run list'
teamcity alias set rw  'run view $1 --web'

# Filtered views
teamcity alias set mine    'run list --user=@me'
teamcity alias set fails   'run list --status=failure --since=24h'
teamcity alias set running 'run list --status=running'

# Trigger-and-watch workflows
teamcity alias set go    'run start $1 --watch'
teamcity alias set hotfix 'run start $1 --top --clean --watch'

# Shell aliases for pipes and external tools
teamcity alias set watchnotify '!teamcity run watch $1 && notify-send "Build $1 done"'
teamcity alias set faillog '!teamcity run list --status=failure --json | jq ".[].id"'
```

**Options:**
- `--shell` – Evaluate expansion as a shell expression via sh

---

## Skills

### skill install

Install the teamcity-cli skill so AI coding agents can use teamcity commands.

Installs globally by default. Use --project to install to the current project only.
Auto-detects installed agents when --agent is not specified.

```bash
teamcity skill install
teamcity skill install --agent claude-code --agent cursor
teamcity skill install --project
```

**Options:**
- `-a, --agent` – Target agent(s); auto-detects if omitted
- `--project` – Install to current project instead of globally

### skill remove

Remove the teamcity-cli skill from AI coding agents

```bash
teamcity skill remove
teamcity skill remove --agent claude-code
teamcity skill remove --project
```

**Options:**
- `-a, --agent` – Target agent(s); auto-detects if omitted
- `--project` – Install to current project instead of globally

### skill update

Update the teamcity-cli skill to the latest version bundled with this teamcity release.

Skips if the installed version already matches.
Auto-detects installed agents when --agent is not specified.

```bash
teamcity skill update
teamcity skill update --agent claude-code
teamcity skill update --project
```

**Options:**
- `-a, --agent` – Target agent(s); auto-detects if omitted
- `--project` – Install to current project instead of globally

<!-- COMMANDS_END -->

</details>

## Awesome Aliases

A collection of useful aliases to get started with:

```bash
# ── Quick shortcuts ─────────────────────────────────────────────
teamcity alias set rl       'run list'                        # List recent runs
teamcity alias set rv       'run view $1'                     # View a run
teamcity alias set rw       'run view $1 --web'               # Open run in browser
teamcity alias set jl       'job list'                        # List jobs
teamcity alias set ql       'queue list'                      # List queued runs

# ── Filtered views ──────────────────────────────────────────────
teamcity alias set mine     'run list --user=@me'             # My runs
teamcity alias set fails    'run list --status=failure --since=24h'  # Recent failures
teamcity alias set running  'run list --status=running'       # What's running now
teamcity alias set morning  'run list --status=failure --since=12h'  # Overnight failures

# ── Trigger-and-watch ──────────────────────────────────────────
teamcity alias set go       'run start $1 --watch'            # Start and watch
teamcity alias set try      'run start $1 --local-changes --watch'  # Test local changes
teamcity alias set hotfix   'run start $1 --top --clean --watch'    # Priority build
teamcity alias set retry    'run restart $1 --watch'          # Re-run a build

# ── Queue management ───────────────────────────────────────────
teamcity alias set rush     'queue top $1'                    # Prioritize a build
teamcity alias set ok       'queue approve $1'                # Approve a queued run

# ── Agent operations ───────────────────────────────────────────
teamcity alias set maint    'agent disable $1'                # Put agent in maintenance
teamcity alias set unmaint  'agent enable $1'                 # Bring agent back

# ── API shortcuts ──────────────────────────────────────────────
teamcity alias set whoami   'api /app/rest/users/current'     # Who am I?

# ── Shell aliases (pipes & external tools) ─────────────────────
teamcity alias set watchnotify '!teamcity run watch $1 && notify-send "Build $1 done"'
teamcity alias set faillog     '!teamcity run list --status=failure --json | jq ".[].id"'
```

## Contributing

Want to help? See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and guidelines.

## License

Apache-2.0
