[//]: # (title: Anonymous usage statistics)

<show-structure for="chapter" depth="2"/>

<tip>

**Just want to opt out?** Set `DO_NOT_TRACK=1` in your shell, or run `teamcity config set analytics false`.

</tip>

TeamCity CLI sends anonymous usage statistics to JetBrains so we can see which commands matter to people, where things break, and where to spend our time next. Every value we send is constrained to a fixed enum or a numeric count – the CLI never transmits free-form strings, identifiers, or anything you typed.

## What we collect

**Once per CLI invocation** (a "session" event):

- CLI version, TeamCity server version (for example, `2026.1`), and server type – `cloud` or `on_prem`.
- OS (`darwin`, `linux`, `windows`, `freebsd`, `other`) and CPU architecture (`amd64`, `arm64`, `386`, `other`).
- The CI system the CLI is running inside, if any: `github_actions`, `gitlab`, `jenkins`, `circleci`, `buildkite`, `azure`, `travis`, `teamcity`, `other`, or `none`.
- The AI coding agent that invoked the CLI, if any: `claude_code`, `junie`, `cursor`, `gemini_cli`, `codex`, `goose`, `augment`, `github_copilot`, `amp`, `windsurf`, `opencode`, `trae`, `roo`, `other`, or `none`.
- How you authenticated: `keyring`, `env`, `build_properties`, `guest`, or `none` – never the token itself.
- Whether the working directory has a `teamcity.toml` linked-project file (a boolean).
- A randomly-generated session ID that rotates after 30 minutes of inactivity. The ID is hashed locally before it leaves your machine.

**Once per command** (a "command" event):

- The command you ran, mapped to a fixed enum: `run.list`, `pipeline.validate`, `agent.term`, and so on. Anything outside the enum collapses to `other`.
- Whether `--json` was used.
- Whether the command ran with git context (one of `--local-changes`, `--branch=@this`, `--revision=@head`).
- Whether a `teamcity.toml` link file was used to resolve the project or job.
- The total number of flags you set (count only – never the values).
- Exit code (`0`, `1`, `2`), duration in milliseconds, and an error category if the command failed: `auth`, `permission`, `not_found`, `network`, `validation`, `read_only`, `internal`, or `none`.

**For specific command groups** (build runs, agent sessions, pipeline operations, skill install/update, REST API calls, login, link) we record a handful of additional behavioral booleans and counts – for example, `is_personal` for `run start`, `error_count` for `pipeline validate`, `had_timeout` for `agent term`. The full, exact schema is committed in [`internal/analytics/scheme.go`](https://github.com/JetBrains/teamcity-cli/blob/main/internal/analytics/scheme.go).

## What we don't collect

- Tokens, passwords, SSH keys, or anything stored in the keyring or `config.yml`.
- Your TeamCity server URL, hostname, or IP address. The URL is used only as a local cache key in `~/.config/tc/.analytics/server-info.json` so the CLI can remember which server version it last saw; the URL itself never leaves your machine.
- Repository contents, build logs, build numbers, or arguments you pass to commands.
- Project, job, build, agent, pipeline, VCS-root, pool, or connection names or IDs.
- Usernames, emails, branch names, file paths, or commit SHAs.

**Nothing we collect can be used to identify you, your organization, your servers, or your repositories.** Every value goes through a published enum or numeric validator before it leaves your machine – the CLI cannot accidentally exfiltrate strings.

## How to opt out

Pick whichever fits your workflow – checked in this order:

**Environment variable** (one-shot or per session):

```Shell
export DO_NOT_TRACK=1
# or, equivalently
export TEAMCITY_ANALYTICS=0
```

`DO_NOT_TRACK` follows the [industry convention](https://donottrack.sh/) used by other CLI tools. Either variable wins over the config file.

**Configuration** (persistent):

```Shell
teamcity config set analytics false
```

To re-enable later:

```Shell
teamcity config set analytics true
```

## Where the data goes

Through the JetBrains FUS (Feature Usage Statistics) pipeline – the same pipeline used by IntelliJ-based IDEs. Retention and processing details: [JetBrains Product Data Collection Terms](https://www.jetbrains.com/legal/docs/terms/product_data_collection/).

<seealso>
    <category ref="user-guide">
        <a href="teamcity-cli-configuration.md">Configuration</a>
    </category>
</seealso>
