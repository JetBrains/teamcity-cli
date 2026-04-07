# Contributing

## Set up your machine

`teamcity` is written in [Go](https://golang.org/).

Prerequisites:

- [Go 1.25+](https://golang.org/doc/install)
- [just](https://github.com/casey/just) (task runner)
- [Docker](https://docs.docker.com/get-docker/) (for integration tests)

Optional:

- [GoLand](https://www.jetbrains.com/go/) or [IntelliJ IDEA](https://www.jetbrains.com/idea/) — both are [free for open-source development](https://www.jetbrains.com/community/opensource/)
- [golangci-lint](https://golangci-lint.run/welcome/install/) (for `just lint`)

Clone and build:

```sh
git clone git@github.com:JetBrains/teamcity-cli.git
cd teamcity-cli
just build
```

## Architecture

```
tc/                  # Entry point (main package)
api/                 # HTTP client — all TeamCity REST API calls live here
  interface.go       # ClientInterface — the contract commands code against
  client.go          # Real HTTP implementation
  types.go           # Request/response structs
internal/
  cmd/               # CLI commands, one subpackage per noun (run/, agent/, project/, …)
    root.go          # Root cobra command, global flags
  cmdutil/           # Shared helpers: client init, view flags, failure summaries
  config/            # Auth config (keyring / file storage)
  output/            # Terminal output: colors, tables, trees, pager, status icons
  errors/            # Structured error types
  terminal/          # Agent WebSocket terminal
acceptance/          # End-to-end .txtar tests against a live server
```

**Data flow:** `tc/main.go` → `cmd.Execute()` builds the cobra tree → each command receives a `*cmdutil.Factory` and calls `f.Client()` to get an `api.ClientInterface` → calls API methods → formats output via `internal/output`.

When adding a new command, create a subpackage under `internal/cmd/<noun>/` and register it in `root.go`.

### Integration tests

Unit tests run without any setup. Integration tests need a TeamCity server — by default, they spin one up via [testcontainers](https://golang.testcontainers.org/), which requires Docker.

To use an existing server instead, copy the env template and fill in your values:

```sh
cp .env.example .env
```

## Development workflow

```sh
just build        # build binary to bin/teamcity
just unit         # run unit tests only
just test         # run all tests (unit + integration) with coverage
just lint         # go fmt + golangci-lint
just docs-generate # regenerate CLI command reference
just install      # install to $GOPATH/bin
```

Run `just` with no arguments to see all available recipes.

## Tests

All new features and bug fixes must include tests. We have a solid integration test setup with testcontainers that spins up a real TeamCity server — please use it. If your change touches API behavior or user-facing commands, an integration test is expected, not just unit tests.

### JSON output contract

All commands that produce data output must support `--json`. When `--json` is active:

- **Success output** goes to stdout as the resource data (object or array).
- **Error output** goes to stderr using the structured `{"error": {"code": "...", "message": "...", "suggestion": "..."}}` envelope. Error classification happens automatically in `root.go` for any command with a `--json` flag.
- **No field removals or renames** without a deprecation period. Additive fields are always safe.
- **New commands** must include `--json` from day one if they produce data output.

See `internal/output/json_error.go` for the error codes and `docs/topics/teamcity-cli-scripting.md` for the full policy.

## Acceptance tests

Acceptance tests are end-to-end blackbox tests that exercise the real CLI binary against a live TeamCity server ([cli.teamcity.com](https://cli.teamcity.com)). They use the [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) framework with declarative `.txtar` scripts in `acceptance/testdata/`.

### Running locally

```sh
just acceptance                    # in-process, guest auth
just snapshot                      # goreleaser snapshot (builds binary + runs acceptance tests)
```

With authentication (runs all tests including write operations):

```sh
TC_ACCEPTANCE_TOKEN=<your-token> just acceptance
```

To run a single test:

```sh
TC_ACCEPTANCE_SCRIPT=agent-cloud go test -tags=acceptance -v ./acceptance/ -count=1 -timeout 10m
```

### How they run in CI

Acceptance tests are embedded in the goreleaser build pipeline as a **post-build hook** (`.goreleaser.yaml`). They run automatically after building the CLI binary for the native platform:

- **Snapshot builds** (every push): guest-auth tests — no token needed
- **Release builds** (tagged): token-auth tests using `TEAMCITY_TOKEN` secret — failures block publishing

### Writing tests

Each `.txtar` file is a self-contained test script. Key patterns:

```
# Tests requiring auth should skip in guest mode
[!has_token] skip 'requires authentication token'

# Run CLI commands
exec teamcity project list --no-input
stdout '.'           # assert stdout contains something
! stderr 'Error'     # assert no errors

# Extract values from JSON output
exec teamcity run start Sandbox_Build --json --no-input
extract '"id":\s*(\d+)' BUILD_ID

# Wait for a cloud agent to be assigned to a build
wait_for_agent $BUILD_ID AGENT_ID
```

Available custom commands: `extract`, `wait_for_agent`, `stdout2env`, `env2upper`, `sleep`.

Available conditions: `[has_token]` (token auth), `[guest]` (guest auth).

### Coverage

Every CLI command and subcommand has acceptance test coverage. The following is intentionally excluded:
- `--web` flags (open a browser, no headless assertion possible)
- `run watch --logs` (starts a full-screen TUI, needs a terminal)
- `agent term` (WebSocket terminal session, needs an interactive TTY)
- `agent enable/disable`, `authorize/deauthorize`, `move`, `reboot` (need admin privileges and a live agent)
- `run start --personal`, `--local-changes`, `--no-push` (need a VCS-connected checkout)
- `project settings validate` (needs Maven installed locally)
- `completion <shell>` (cobra has it tested)

**Flags tested implicitly** (same code path as tested flags):
- `--secure` on `param set` (identical to a regular set, just marks value encrypted server-side)
- `run start --rebuild-deps`, `--agent`, `--rebuild-failed-deps`, `--clean` (build queue options, same API path as `--branch`)

If you add a new command, add a corresponding `.txtar` test in `acceptance/testdata/<command>/`.

### Test environment

- **Server**: `cli.teamcity.com` (TeamCity Cloud, configurable via `TC_ACCEPTANCE_HOST`)
- **Sandbox project**: use `Sandbox` for any write operations (param set/delete, token put, run start)
- **Cloud agents**: ephemeral — tests that need agents must start a build, wait for assignment, then clean up
- **Isolation**: each test gets its own `HOME` directory, no cross-test state leakage

## AI-assisted contributions

We're fine with AI tools — Junie, Claude Code, Copilot, whatever helps you move faster. But you must understand the code you're submitting. `teamcity` is a tool where we prioritize security and reliability. PRs with AI-generated code that the author can't explain or defend during review will not be merged.

## Documentation

The canonical documentation lives in [JetBrains/teamcity-documentation](https://github.com/JetBrains/teamcity-documentation) and is published at [jb.gg/tc/docs](https://jb.gg/tc/docs). A local copy is kept in `docs/topics/` for reference and editing convenience.

Use the sync recipes to keep local and upstream docs in sync:

```sh
just docs-pull              # fetch latest from teamcity-documentation
just docs-push              # open a PR to teamcity-documentation with local changes
just docs-generate          # regenerate the CLI command reference table
```

When your change adds or modifies commands, flags, or user-facing behavior, update **all** of the following:

| Location               | What to update                                                                   |
|------------------------|----------------------------------------------------------------------------------|
| `docs/topics/`         | Writerside topic files (`.md`) — edit locally, then `just docs-push` to upstream |
| `skills/teamcity-cli/` | AI agent skill — `SKILL.md`, `references/commands.md`, `references/workflows.md` |
| `README.md`            | Commands table in the root readme                                                |

**GIFs:** Terminal recordings (in `docs/images/`) illustrate key workflows. If your change visibly alters CLI output for an existing GIF, re-record it. Use [vhs](https://github.com/charmbracelet/vhs) with tape files in `docs/tapes/`.

**Keep docs in sync:** It's easy to forget one of the locations above. A good check: grep for the flag or command name you changed across `docs/`, `skills/`, and `README.md` to make sure nothing is stale.

## Flags and short-flag conventions

Follow these rules when adding flags:

**Reserved short flags.** These are taken globally and must never be reused by subcommands:

| Short | Global flag                  |
|-------|------------------------------|
| `-q`  | `--quiet`                    |
| `-v`  | `--version` (Cobra built-in) |

**Don't shadow globals.** A subcommand flag like `--verbose` with `-v` shadows Cobra's built-in `--version`. A subcommand `-q` shadows the global `--quiet`. If in doubt, skip the short flag entirely — a long flag with no shorthand is always safe.

**Avoid ambiguous shorthands.** If a command has both `--limit` (`-n`) and `--dry-run`, don't give `-n` to `--dry-run` — it conflicts. When two flags could reasonably claim the same letter, neither gets it.

**Use standard flag names.** Prefer these established names for consistency across commands:

| Meaning                       | Flag name  | Short         |
|-------------------------------|------------|---------------|
| Limit number of results       | `--limit`  | `-n`          |
| Filter by branch              | `--branch` | `-b`          |
| Skip confirmation prompt      | `--force`  | `-f`          |
| JSON output                   | `--json`   | —             |
| Suppress non-essential output | `--quiet`  | `-q` (global) |

## Deprecating flags and commands

### Flags

When renaming or retiring a flag, use `cmdutil.DeprecateFlag`:

```go
cmd.Flags().StringVar(&opts.job, "job", "", "Filter by job")
cmd.Flags().StringVar(&opts.job, "build-type", "", "")
cmdutil.DeprecateFlag(cmd, "build-type", "job", "v2.0")
```

Stderr when `--build-type` is used:
```
Flag --build-type has been deprecated, use --job instead (will be removed in v2.0)
```

Rules:
- Register the old flag **before** calling `DeprecateFlag` — it panics if the flag is not found (catches typos at startup)
- Bind the old flag to the **same variable** as the new flag so both work
- Set the old flag's usage to `""` — Cobra hides deprecated flags from `--help` automatically
- Pick a removal version at least one minor release out

### Commands

When retiring or replacing a command, use `cmdutil.DeprecateCommand`:

```go
cmd := &cobra.Command{Use: "old-cmd", ...}
cmdutil.DeprecateCommand(cmd, "new-cmd", "v2.0")
```

Stderr when `old-cmd` is invoked:
```
Command old-cmd is deprecated, use "new-cmd" instead (will be removed in v2.0)
```

The command still runs — users are warned but not broken. Remove it in the target version.

No flags or commands are deprecated today; these are the patterns for when the first deprecation is needed.

## Submit a pull request

Push your branch and open a PR against `main`. The [PR template](.github/PULL_REQUEST_TEMPLATE.md) will guide you through describing the change.

Before submitting, make sure:

- `just lint` passes
- `just unit` passes (at minimum)
- You've manually tested your change

## Release a new version

> This section is for maintainers.

Releases are handled by [goreleaser](https://goreleaser.com/) and publish to Homebrew, Scoop, Chocolatey, Winget, and GitHub Releases.

### Dry-run locally

```sh
just snapshot         # build a local snapshot
just release-dry-run  # full release process without publishing
```

### Cutting a release

Tag and push — the [release pipeline](https://teamcity-nightly.labs.intellij.net/pipeline/TeamCity_TeamCityCLI_Release) on TeamCity handles everything else automatically (build, acceptance test, sign, publish to all package managers):

```sh
git tag -a v1.1.1 -m "Release v1.1.1"
git push origin v1.1.1
```

### Rolling back a release

If a release needs to be reverted:

1. Revert the formula/manifest commits in [jetbrains/homebrew-utils](https://github.com/JetBrains/homebrew-utils) and [jetbrains/scoop-utils](https://github.com/JetBrains/scoop-utils)
2. Close the auto-created winget PR in [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs)
3. Cancel the Chocolatey submission (if still pending moderation) on [chocolatey.org](https://community.chocolatey.org/)
4. Delete the tag and release it from the [GitHub repository](https://github.com/JetBrains/teamcity-cli):
   ```sh
   git tag -d v1.1.1
   git push origin --delete v1.1.1
   ```
   Then delete the release from the [Releases page](https://github.com/JetBrains/teamcity-cli/releases).
