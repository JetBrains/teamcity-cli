[//]: # (title: Getting Started)

<show-structure for="chapter" depth="2"/>

This guide walks you through installing TeamCity CLI, authenticating with a TeamCity server, and running your first commands.

## Install TeamCity CLI {id="install"}

### Prerequisites

TeamCity CLI requires a running TeamCity server (version 2020.1 or later) to connect to. Some features may require newer TeamCity versions (for example, 2024.04 or later). No additional runtime dependencies are needed — the CLI is distributed as a standalone binary.

### Installation {id="installation"}

<tabs>
<tab title="macOS">

**Homebrew (recommended):**

```Shell
brew install jetbrains/utils/teamcity
```

To update to the latest version:

```Shell
brew upgrade teamcity
```

**Install script:**

```Shell
curl -fsSL https://jb.gg/tc/install | bash
```

The script detects your operating system and architecture automatically and installs the `teamcity` binary to a directory on your PATH.

> If your environment disallows piping scripts to a shell, use the platform packages or download a release artifact instead.
>
{style="note"}

</tab>
<tab title="Linux">

**Install script (all distros):**

```Shell
curl -fsSL https://jb.gg/tc/install | bash
```

The script detects your operating system and architecture automatically and installs the `teamcity` binary to a directory on your PATH.

> If your environment disallows piping scripts to a shell, use the platform packages instead.
>
{style="note"}

**Packages by distribution:**

<tabs>
<tab title="Debian/Ubuntu">

```Shell
curl -fsSLO https://github.com/JetBrains/teamcity-cli/releases/latest/download/teamcity_linux_amd64.deb
sudo dpkg -i teamcity_linux_amd64.deb
```

</tab>
<tab title="RHEL/Fedora">

```Shell
sudo rpm -i https://github.com/JetBrains/teamcity-cli/releases/latest/download/teamcity_linux_amd64.rpm
```

</tab>
<tab title="Arch">

```Shell
curl -fsSLO https://github.com/JetBrains/teamcity-cli/releases/latest/download/teamcity_linux_amd64.pkg.tar.zst
sudo pacman -U teamcity_linux_amd64.pkg.tar.zst
```

</tab>
</tabs>

</tab>
<tab title="Windows">

**Winget (recommended):**

```PowerShell
winget install JetBrains.TeamCityCLI
```

**PowerShell (install script):**

```PowerShell
irm https://jb.gg/tc/install.ps1 | iex
```

**CMD (install script):**

```Shell
curl -fsSL https://jb.gg/tc/install.cmd -o install.cmd && install.cmd && del install.cmd
```

**Chocolatey:**

```PowerShell
choco install teamcitycli
```

**Scoop:**

```PowerShell
scoop bucket add jetbrains https://github.com/JetBrains/scoop-utils
scoop install teamcity
```

</tab>
</tabs>

<chapter title="Build from source (advanced)" collapsible="true" default-state="collapsed">

> Released binaries are tested, signed, and verified. For production use, prefer a released version from the options above.
>
{style="warning"}

**Go install:**

```Shell
go install github.com/JetBrains/teamcity-cli/tc@latest
```

**Clone and build:**

```Shell
git clone https://github.com/JetBrains/teamcity-cli.git
cd teamcity-cli
go build -o teamcity ./tc
```

</chapter>

### Verify the installation {id="verify"}

After installing, verify that the CLI is available:

```Shell
teamcity --version
```

## Authenticate with your server {id="authenticate"}

1. Run the login command:

   ```Shell
   teamcity auth login
   ```

2. Follow the prompts to enter the server URL and create an access token in your browser.

3. Verify the login:

   ```Shell
   teamcity auth status
   ```

Tokens are stored in your system keyring (macOS Keychain, GNOME Keyring, or Windows Credential Manager) when available.

> If your system does not have a keyring, the CLI falls back to storing the token in the configuration file. You can force this behavior with `--insecure-storage`.
>
{style="note"}

### Guest access {id="guest-access"}

If the server has guest access enabled, you can log in without a token:

```Shell
teamcity auth login --guest
```

Guest access provides read-only access to the server.

<img src="auth-login.gif" alt="Authentication status and guest login" border-effect="rounded"/>

## Understand the terminology {id="terminology"}

TeamCity CLI uses shorter names for TeamCity concepts. Knowing these mappings will help you navigate the commands.

<deflist type="medium">
    <def title="Run" id="term-run">
        A single build execution. Equivalent to <b>build</b> in the TeamCity web interface. Run IDs are numeric.
        <code-block lang="Shell">teamcity run list</code-block>
    </def>
    <def title="Job" id="term-job">
        A build configuration — the set of instructions that define how to run a build. Job IDs look like <code>MyProject_Build</code>.
        <code-block lang="Shell">teamcity job list</code-block>
    </def>
    <def title="Project" id="term-project">
        A collection of jobs. Projects can be nested to form a hierarchy. Project IDs look like <code>MyProject</code>.
        <code-block lang="Shell">teamcity project list</code-block>
    </def>
</deflist>

The hierarchy is: **Project** contains **Jobs**, each Job produces **Runs**, and each Run executes on an **Agent**.

> Most commands expect IDs, not display names. Use `teamcity job list` or `teamcity project list` to find them. See the [Glossary](teamcity-cli-glossary.md) for the full terminology mapping.
>
{style="tip"}

## List recent builds {id="list-builds"}

```Shell
teamcity run list
```

Add filters to narrow results:

```Shell
# Builds from a specific job
teamcity run list --job MyProject_Build

# Only failures from the last 24 hours
teamcity run list --status failure --since 24h

# Builds on a specific branch
teamcity run list --branch main --limit 10
```

<img src="run-list.gif" alt="Listing and filtering runs" border-effect="rounded"/>

## Find job IDs {id="find-job-ids"}

Many commands require a job ID. Use `teamcity job list` to browse available jobs:

```Shell
# List all jobs
teamcity job list

# Filter by project
teamcity job list --project MyProject
```

<img src="job-list.gif" alt="Finding job IDs" border-effect="rounded"/>

> Job IDs like `MyProject_Build` are not the same as display names. Always use the ID column from the output.
>
{style="note"}

## Start a build {id="start-build"}

Trigger a new build by specifying a job ID:

```Shell
teamcity run start MyProject_Build
```

Add `--watch` to follow the build in real time:

```Shell
teamcity run start MyProject_Build --branch main --watch
```

The `--watch` flag displays a live progress view that updates until the build completes.

<img src="run-start-watch.gif" alt="Starting a build with --watch" border-effect="rounded"/>

## View build logs {id="view-logs"}

View the log output from a specific build:

```Shell
teamcity run log 12345
```

Or get the latest log for a job:

```Shell
teamcity run log --job MyProject_Build
```

<img src="run-log.gif" alt="Viewing build logs in the pager" border-effect="rounded"/>

> The log viewer opens in a pager. Use <shortcut>/</shortcut> to search, <shortcut>n</shortcut>/<shortcut>N</shortcut> to navigate matches, <shortcut>g</shortcut>/<shortcut>G</shortcut> to jump to top/bottom, and <shortcut>q</shortcut> to quit. Pass `--raw` to bypass the pager.
>
{style="tip"}

## Investigate a failure {id="investigate-failure"}

When a build fails, use this workflow to quickly find the root cause:

1. Find the failed build:

   ```Shell
   teamcity run list --status failure
   ```

2. View failure diagnostics (problems, failed tests with full stack traces):

   ```Shell
   teamcity run log 12345 --failed
   ```

3. Inspect individual test failures:

   ```Shell
   teamcity run tests 12345 --failed
   ```

<img src="run-tests.gif" alt="Viewing failed test results" border-effect="rounded"/>

> You can combine these steps: find failures with `run list --status failure`, then jump straight to `run log <id> --failed` for the most common investigation path.
>
{style="tip"}

## Check the build queue {id="build-queue"}

See what builds are waiting to run:

```Shell
teamcity queue list
```

## View build agents {id="build-agents"}

List all registered build agents and their status:

```Shell
teamcity agent list
```

Filter to show only connected agents:

```Shell
teamcity agent list --connected
```

<img src="agent-list.gif" alt="Listing build agents" border-effect="rounded"/>

## Open in the browser {id="open-in-browser"}

Most view commands support a `--web` flag that opens the corresponding page in your browser:

```Shell
teamcity run view 12345 --web
teamcity job view MyProject_Build --web
teamcity project view MyProject --web
```

> The `--web` flag works with `run view`, `job view`, `project view`, and `agent view`.
>
{style="tip"}

## Next steps {id="next-steps"}

<deflist>
    <def title="Shell completion">
        Set up tab completion for Bash, Zsh, Fish, or PowerShell — see <a href="teamcity-cli-configuration.md#Shell+completion">Configuration</a>.
    </def>
    <def title="Authentication">
        Learn about <a href="teamcity-cli-authentication.md">authentication methods</a> including multi-server setup and CI/CD usage.
    </def>
    <def title="Managing runs">
        Go deeper with <a href="teamcity-cli-managing-runs.md">build management</a> — artifacts, personal builds, pinning, tagging, and more.
    </def>
    <def title="Aliases">
        Set up <a href="teamcity-cli-aliases.md">custom shortcuts</a> for frequently used commands.
    </def>
    <def title="Scripting">
        Configure <a href="teamcity-cli-scripting.md">JSON output</a> for scripting and automation.
    </def>
    <def title="Command reference">
        Browse the full <a href="teamcity-cli-commands.md">command reference</a> for all available commands and flags.
    </def>
</deflist>

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
        <a href="teamcity-cli-configuration.md">Configuration</a>
        <a href="teamcity-cli-glossary.md">Glossary</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-managing-runs.md">Managing runs</a>
        <a href="teamcity-cli-managing-jobs.md">Managing jobs</a>
        <a href="teamcity-cli-aliases.md">Aliases</a>
        <a href="teamcity-cli-scripting.md">Scripting and automation</a>
    </category>
</seealso>
