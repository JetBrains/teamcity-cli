# TeamCity CLI

[![](https://camo.githubusercontent.com/078d7efd31e09afaa403fc886eac57d43ece79ad24fb75be8e05ac2b13175bef/68747470733a2f2f6a622e67672f6261646765732f6f6666696369616c2d706c61737469632e737667)](https://github.com/JetBrains)
[![GitHub Release](https://img.shields.io/github/v/release/JetBrains/teamcity-cli?style=plastic)](https://github.com/JetBrains/teamcity-cli/releases/latest)

TeamCity CLI (`teamcity`) is an open-source command-line interface for [TeamCity](https://www.jetbrains.com/teamcity/). Start builds, tail logs, manage agents and queues – without leaving your terminal.

> **[Documentation](https://jb.gg/tc/docs)** – full guide with installation, authentication, and command reference.

![cli](https://github.com/user-attachments/assets/fa6546f2-5630-4116-aa6c-5addc8d83318)

## Features

- **Stay in your terminal** – Start builds, view logs, manage queues – no browser needed
- **Remote agent access** – Shell into any build agent with `teamcity agent term`, or run commands with `teamcity agent exec`
- **Real-time logs** – Stream build output as it happens with `teamcity run watch --logs`
- **Scriptable** – `--json` and `--plain` output for pipelines, plus direct REST API access via `teamcity api`
- **Multi-server support** – Authenticate with and switch between multiple TeamCity instances
- **AI agent ready** – Built-in [skill](https://agentskills.io) for Claude Code, Cursor, and other AI coding agents – just run `teamcity skill install`

## Installation

**macOS (Homebrew):**
```bash
brew install jetbrains/utils/teamcity
```

**Linux:**
```bash
curl -fsSL https://jb.gg/tc/install | bash
```

**Windows (Winget):**
```powershell
winget install JetBrains.TeamCityCLI
```

<details>
<summary>More installation methods (deb, rpm, Chocolatey, Scoop, build from source)</summary>

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
choco install teamcitycli
```

**Scoop:**
```powershell
scoop bucket add jetbrains https://github.com/JetBrains/scoop-utils
scoop install teamcity
```

**Build from source:**
```bash
go install github.com/JetBrains/teamcity-cli/tc@latest
```

See the [getting started guide](https://jetbrains.github.io/teamcity-cli/teamcity-cli-get-started.html) for the full walkthrough.

</details>

## Quick start

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

> **Note:** The CLI uses "run" for builds and "job" for build configurations. See the [glossary](https://jetbrains.github.io/teamcity-cli/teamcity-cli-glossary.html) for the full mapping.

## Commands

| Group       | Commands                                                                                                                                           |
|-------------|----------------------------------------------------------------------------------------------------------------------------------------------------|
| **auth**    | `login`, `logout`, `status`                                                                                                                        |
| **run**     | `list`, `start`, `view`, `watch`, `log`, `changes`, `tests`, `cancel`, `download`, `artifacts`, `restart`, `pin`/`unpin`, `tag`/`untag`, `comment` |
| **job**     | `list`, `view`, `pause`/`resume`, `param list`/`get`/`set`/`delete`                                                                                |
| **project** | `list`, `view`, `param`, `token get`/`put`, `settings export`/`status`/`validate`                                                                  |
| **queue**   | `list`, `approve`, `remove`, `top`                                                                                                                 |
| **agent**   | `list`, `view`, `term`, `exec`, `jobs`, `authorize`/`deauthorize`, `enable`/`disable`, `move`, `reboot`                                            |
| **pool**    | `list`, `view`, `link`/`unlink`                                                                                                                    |
| **api**     | Raw REST API access                                                                                                                                |
| **alias**   | `set`, `list`, `delete`                                                                                                                            |
| **skill**   | `install`, `remove`, `update`                                                                                                                      |

Run `teamcity <command> --help` for usage details. See the [command reference](https://jetbrains.github.io/teamcity-cli/teamcity-cli-commands.html) for full documentation.

## For AI agents

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

See [AI agent integration](https://jetbrains.github.io/teamcity-cli/teamcity-cli-ai-agent-integration.html) for details.

## Learn more

- [Getting started](https://jetbrains.github.io/teamcity-cli/teamcity-cli-get-started.html) – install, authenticate, and run your first commands
- [Configuration](https://jetbrains.github.io/teamcity-cli/teamcity-cli-configuration.html) – config file, environment variables, multi-server setup, shell completion
- [Scripting and automation](https://jetbrains.github.io/teamcity-cli/teamcity-cli-scripting.html) – JSON output, plain text mode, CI/CD integration
- [Aliases](https://jetbrains.github.io/teamcity-cli/teamcity-cli-aliases.html) – create custom command shortcuts

## Contributing

TeamCity CLI is open source under the Apache-2.0 license. Contributions are welcome – see [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and guidelines.

## License

Apache-2.0
