# TeamCity CLI

[![](https://camo.githubusercontent.com/078d7efd31e09afaa403fc886eac57d43ece79ad24fb75be8e05ac2b13175bef/68747470733a2f2f6a622e67672f6261646765732f6f6666696369616c2d706c61737469632e737667)](https://github.com/JetBrains)
[![GitHub Release](https://img.shields.io/github/v/release/JetBrains/teamcity-cli?style=plastic)](https://github.com/JetBrains/teamcity-cli/releases/latest)

`teamcity` is the official command-line client for [TeamCity](https://www.jetbrains.com/teamcity/). Use it to start builds, tail logs, and check the queue, as well as for less frequent tasks such as opening a shell on a build agent, editing job settings, or calling the REST API directly.

![cli](./docs/images/showcase_dark.gif#gh-dark-mode-only)
![cli](./docs/images/showcase.gif#gh-light-mode-only)

Full documentation, including installation, authentication, and a command reference, is at [jb.gg/tc/docs](https://jb.gg/tc/docs).

## Install

**macOS (Homebrew):**
```bash
brew install jetbrains/utils/teamcity
```

**Linux:**
```bash
curl -fsSL https://jb.gg/tc/install | bash
```

The install script also works on macOS. New installs use `~/.local/bin` without sudo; rerunning it updates an existing unmanaged installation in place. Missing Bash/Zsh PATH entries are added automatically. Restart your shell afterward, or use the printed PATH command.

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

**Arch Linux (AUR):**
```bash
yay -S teamcity-bin
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

See the [getting started guide](https://www.jetbrains.com/help/teamcity/teamcity-cli-get-started.html) for the full walkthrough.

</details>

## Usage

Log in once and the CLI remembers the server:

```bash
teamcity auth login
```

Scripts and desktop apps can sign in through the browser without a terminal: `teamcity auth login --server <url> --web --json` never prompts and prints the authorization URL as a JSON line.

Then:

```bash
# my latest build on this branch
teamcity run list --user @me --branch @this --limit 1

# start a build and stay attached to it
teamcity run start MyProject_Build --branch main --watch

# pin different revisions in a job with multiple VCS roots
teamcity run start MyProject_Build --revision AppRepo=abc123 --revision AssetsRepo=@main

# logs from the latest build of a job
teamcity run log --job MyProject_Build

# what's sitting in the queue
teamcity queue list

# shell into a build agent
teamcity agent term Agent-Linux-01
```

TeamCity says *build* and *build configuration*; the CLI says `run` and `job`. The [glossary](https://www.jetbrains.com/help/teamcity/teamcity-cli-glossary.html) has the full mapping.

Set `TEAMCITY_RO=1` to block writes and remote shells (`agent exec` and `agent term`). Use server-side permissions for a security boundary. Redirects never downgrade HTTPS or forward request headers to another origin, and cross-origin artifact downloads are made without credentials.

Every command takes `--json` or `--plain` for [scripting](https://www.jetbrains.com/help/teamcity/teamcity-cli-scripting.html), and `--web` opens the matching page in the TeamCity UI. When no command covers what you need, `teamcity api` calls the REST API directly with your stored credentials. You can also log in to several servers and switch between them; see [configuration](https://www.jetbrains.com/help/teamcity/teamcity-cli-configuration.html).

For a queued run, `teamcity run view <id>` reports agent and cloud-image compatibility, including unmet requirements, when the server supports it. Large pools are summarized; use `--json` and read `.compatibility` for every entry.

`teamcity run download` writes only inside `--output`, and a failed or incomplete transfer leaves existing files untouched.

## Commands

| Group        | Commands                                                                                                                                                                                                                                                                                                        |
|--------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **auth**     | `login`, `logout`, `status`                                                                                                                                                                                                                                                                                     |
| **run**      | `list`, `start`, `view`, `watch`, `log`, `tree`, `changes`, `tests`, `diff`, `cancel`, `download`, `artifacts`, `restart`, `pin`/`unpin`, `tag`/`untag`, `comment`                                                                                                                                              |
| **job**      | `list`, `view`, `create`, `tree`, `pause`/`resume`, `step list`/`view`/`add`/`delete`, `param list`/`get`/`set`/`delete`, `settings list`/`get`/`set`                                                                                                                                                           |
| **project**  | `list`, `view`, `create`, `tree`, `vcs list`/`view`/`create`/`test`/`delete`, `ssh list`/`generate`/`upload`/`delete`, `cloud profile`/`image`/`instance`, `connection list`/`view`/`create github-app`/`create docker`/`authorize`/`delete`, `param`, `token get`/`put`, `settings export`/`status`/`validate` |
| **pipeline** | `list`, `view`, `create`, `validate`, `pull`, `push`, `schema` (enabled runners/features), `delete`                                                                                                                                                                                                                                        |
| **queue**    | `list`, `approve`, `remove`, `top`                                                                                                                                                                                                                                                                              |
| **agent**    | `list`, `view`, `term`, `exec`, `jobs`, `authorize`/`deauthorize`, `enable`/`disable`, `move`, `reboot`                                                                                                                                                                                                         |
| **pool**     | `list`, `view`, `link`/`unlink`                                                                                                                                                                                                                                                                                 |
| **server**   | `plugin upload` (optionally with `--hot-reload`)                                                                                                                                                                                                                                                                |
| **api**      | Raw REST API access                                                                                                                                                                                                                                                                                             |
| **link**     | Bind this repository to a TeamCity project via `teamcity.toml`                                                                                                                                                                                                                                                  |
| **config**   | `list`, `get`, `set`                                                                                                                                                                                                                                                                                            |
| **alias**    | `set`, `list`, `delete`                                                                                                                                                                                                                                                                                         |
| **skill**    | `list`, `install`, `remove`, `update`, `guide`                                                                                                                                                                                                                                                                  |
| **update**   | Check for and install CLI updates (`--check`/`--json` for status only, `--yes` for non-interactive installation) |

Run `teamcity <command> --help` for usage, or see the [command reference](https://www.jetbrains.com/help/teamcity/teamcity-cli-commands.html).

## For AI agents

The CLI ships with an [Agent Skill](https://agentskills.io) that teaches coding agents (Claude Code, Cursor, and others) how to drive `teamcity`:

```bash
teamcity skill install           # auto-detects installed agents
teamcity skill install --project # install to current project only
teamcity skill update            # update to the version bundled with teamcity
teamcity skill remove            # uninstall
teamcity skill guide             # print the guide that installed skills point to
```

Installed skills are short stubs that tell the agent to run `teamcity skill guide`, so after a CLI upgrade agents read the instructions for the new version without reinstalling.

For **Claude Code**, you can install the `teamcity` plugin instead, which
bundles these skills from
[JetBrains/teamcity-skills](https://github.com/JetBrains/teamcity-skills):

```bash
/plugin marketplace add JetBrains/teamcity-skills
/plugin install teamcity@teamcity
```

The plugin contains only the skills, so install the `teamcity` CLI itself as described above.

See [AI agent integration](https://www.jetbrains.com/help/teamcity/teamcity-cli-ai-agent-integration.html) for details.

## Learn more

- [Getting started](https://www.jetbrains.com/help/teamcity/teamcity-cli-get-started.html): install, authenticate, and run your first commands
- [Configuration](https://www.jetbrains.com/help/teamcity/teamcity-cli-configuration.html): config file, environment variables, multi-server setup, shell completion
- [Scripting and automation](https://www.jetbrains.com/help/teamcity/teamcity-cli-scripting.html): JSON output, plain text mode, CI/CD integration
- [Aliases](https://www.jetbrains.com/help/teamcity/teamcity-cli-aliases.html): create custom command shortcuts

## Contributing

TeamCity CLI is open source under the Apache-2.0 license. Bug reports and pull requests are welcome. [CONTRIBUTING.md](CONTRIBUTING.md) explains how to build and test the project.
