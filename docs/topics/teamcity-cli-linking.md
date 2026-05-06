[//]: # (title: Linking a repository)

<show-structure for="chapter" depth="2"/>

`teamcity link` binds a repository to one or more TeamCity projects and jobs by writing a small `teamcity.toml` file at the repo root. Once linked, commands like `teamcity run start`, `teamcity run watch`, and `teamcity job list` pick up the default project and job automatically — no `--project`/`--job` flags required.

`teamcity.toml` is meant to be committed alongside your code. It is portable across machines, CI agents, and AI coding agents, and it supports monorepos with per-path bindings and multi-server setups.

## Linking the repo

Run `teamcity link` from the repo root. With no flags it picks up the active server (see [`teamcity auth`](teamcity-cli-authentication.md)) and prompts for the project and default job interactively:

```Shell
teamcity link
```

To skip the prompts, pass the IDs explicitly:

```Shell
teamcity link --project Acme_Backend --job Acme_Backend_Build
```

The first run creates `teamcity.toml`. Subsequent runs upsert the matching `[[server]]` entry — fields you do not pass are preserved.

### Auto-discover from git remotes

In CI or AI-agent contexts where prompts are not an option, `--auto` infers the binding from the repo's git remotes:

```Shell
teamcity link --auto
```

The CLI matches your `origin` URL against VCS roots on the active server and picks the project (and a default job) without any user input. Pass `--server` if you have multiple servers authenticated:

```Shell
teamcity link --auto --server https://nightly.example
```

`--auto` is mutually exclusive with `--project`, `--job`, and `--jobs`.

## The teamcity.toml file

A minimal `teamcity.toml` for a single repo and single server looks like this:

```TOML
[[server]]
url = "https://teamcity.example.com"
project = "Acme_Backend"
job = "Acme_Backend_Build"
```

`url` identifies the TeamCity instance; `project` and `job` are the default IDs used when the corresponding flag is omitted on a command. Use `jobs` (an array) to track multiple jobs of interest:

```TOML
[[server]]
url = "https://teamcity.example.com"
project = "Acme_Backend"
job = "Acme_Backend_Build"
jobs = ["Acme_Backend_Build", "Acme_Backend_Deploy"]
```

`teamcity.toml` is plain TOML — you can edit it by hand, inspect it with `cat teamcity.toml`, or remove it with `rm teamcity.toml`.

## Monorepos: per-path scopes

In a monorepo, each top-level directory often maps to a different project on TeamCity. Run `teamcity link` from inside a subdirectory and the CLI scopes the binding to that path:

```Shell
cd services/api
teamcity link --project Acme_API --job Acme_API_Build

cd ../web
teamcity link --project Acme_Web --job Acme_Web_Build
```

The resulting `teamcity.toml` (always written at the repo root) groups the path scopes under the parent server entry:

```TOML
[[server]]
url = "https://teamcity.example.com"
project = "Acme_Backend"           # repo-wide default
job = "Acme_Backend_Build"

  [server.paths."services/api"]
  project = "Acme_API"
  job = "Acme_API_Build"

  [server.paths."services/web"]
  project = "Acme_Web"
  jobs = ["Acme_Web_Build", "Acme_Web_Deploy"]
```

When you run a CLI command from `services/api`, the `services/api` scope wins. From `services/web/src`, the deepest matching scope (`services/web`) is used. From the repo root, the top-level fields apply.

To force a write at the top-level scope from inside a subdirectory, pass `--scope=`:

```Shell
teamcity link --project Acme_Backend --job Acme_Backend_Build --scope=
```

## Multiple servers

A single `teamcity.toml` can list multiple `[[server]]` entries — useful when, for example, your nightly pipelines run on a separate instance:

```Shell
teamcity link --server https://nightly.example \
    --project Acme_Nightly \
    --jobs Acme_Nightly_Release,Acme_Nightly_Eval
```

Each `--server` is upserted independently:

```TOML
[[server]]
url = "https://teamcity.example.com"
project = "Acme_Backend"
job = "Acme_Backend_Build"

[[server]]
url = "https://nightly.example"
project = "Acme_Nightly"
jobs = ["Acme_Nightly_Release", "Acme_Nightly_Eval"]
```

Switch between servers per command with `--server`, or set `TEAMCITY_URL` for the duration of a shell session.

## Resolution cascade

When a command resolves a project or job, the CLI consults the following sources, highest priority first:

1. Explicit flags on the command (`--project`, `--job`, `--server`, …)
2. `TEAMCITY_*` environment variables
3. The `[[server]]` entry that matches the active server URL, drilling down to the deepest matching `[server.paths."..."]` scope based on your current working directory
4. Active server defaults (from `teamcity auth login`)

This means `teamcity.toml` provides the *defaults*, but any explicit flag or env var still wins.

## Commands that use the link

Once `teamcity.toml` is in place, the following commands accept the linked defaults instead of requiring identifiers on every invocation:

- `teamcity run start` — uses the default `job`
- `teamcity run list`, `teamcity run watch`, `teamcity run log` — accept `--job` from the link
- `teamcity job list`, `teamcity job tree` — scope to the linked `project`
- `teamcity project view`, `teamcity project tree` — open the linked `project`
- `teamcity pipeline pull`, `teamcity pipeline validate` — use the linked `project`

Run `teamcity <command> --help` to see which flags accept the linked default.

<seealso>
    <category ref="user-guide">
        <a href="teamcity-cli-authentication.md">Authentication</a>
        <a href="teamcity-cli-configuration.md">Configuration</a>
        <a href="teamcity-cli-managing-runs.md">Managing runs</a>
    </category>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
</seealso>
