# Contributing

## Set up your machine

`teamcity` is written in [Go](https://golang.org/).

Prerequisites:

- [Go 1.26+](https://golang.org/doc/install)
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

On Windows, `just build`, `just install`, `just lint`, and the other simple Go recipes work in PowerShell with no extra setup. A handful of recipes use bash shebangs (`clean`, `docs-build`, `docs-deploy`, `install-choco`, `install-codesign`, `eval`, `eval-diff`) and require Git Bash or WSL.

### Development workflow

```sh
just build          # go build → bin/teamcity
just install        # go install ./tc → $GOPATH/bin/teamcity
just lint           # go fmt + go fix + golangci-lint
just unit           # unit tests
just test           # unit + integration (testcontainers)
just acceptance     # e2e against cli.teamcity.com (-tags=acceptance)
just snapshot       # goreleaser local snapshot (all platforms)
just docs-generate  # regenerate CLI command reference
just record-gifs <name>  # record GIF from docs/tapes/<name>.tape → docs/images/
```

Run `just` with no arguments to see all available recipes.

### Building

The main package is `./tc/`, not the repo root:

```sh
go build -o bin/teamcity ./tc/
go install ./tc/
```

`go build .` from the repo root produces an `ar` archive — the root is `package teamcitycli` (skills embed), not `main`.

### Integration tests

Unit tests run without any setup. Integration tests need a TeamCity server — by default, they spin one up via [testcontainers](https://golang.testcontainers.org/), which requires Docker.

To use an existing server instead, copy the env template and fill in your values:

```sh
cp .env.example .env
```

## Architecture

```
tc/                  # main package
api/                 # public HTTP client — don't break exported interface
  interface.go       # ClientInterface
  client.go          # HTTP implementation
  types.go           # request/response structs
internal/
  cmd/               # one subpackage per noun: run/, agent/, project/, job/, …
    root.go          # root cobra command, global flags
  cmdutil/           # Factory, shared helpers, client init
  cmdtest/           # mock server, RunCmdWithFactory, SetupMockClient
  config/            # auth (keyring + file), server detection
  output/            # Printer, colors, tables, trees, status icons
  errors/            # Structured error types
  terminal/          # Agent WebSocket terminal
acceptance/          # .txtar e2e tests (testscript framework)
docs/                # Writerside topics + images + tapes
skills/teamcity-cli/ # AI agent skill
```

**Data flow:** `tc/main.go` → `cmd.Execute()` → cobra tree → `*cmdutil.Factory` → `f.Client()` → API → `output.Printer`.

Every command:
1. Package under `internal/cmd/<noun>/`
2. `NewCmd(f *cmdutil.Factory)` returns cobra command
3. `run<Verb>(f, opts)` does the work — pure logic, testable
4. Register in `root.go`

### `api/` is public

Breaking changes to exported types/functions need explicit sign-off. `internal/` refactoring is free.

### Before adding a new package

Search for the helper you think you need before creating a new package. `internal/cmd/<sub>/git.go`, `internal/cmdutil/`, etc. may already host it. Creating a parallel package (e.g. duplicating `isGitRepo`) is a common trap and gets caught in review — extract a shared package only when there's a second consumer.

## Go conventions

Go 1.26. Follow [JetBrains Go Modern Guidelines](https://github.com/JetBrains/go-modern-guidelines).

Hard rules:
- **No CGO.** Any dep requiring CGO is rejected.
- **No `os.Exit` in commands.** Return errors; only `tc/main.go` exits.
- **`[]T{}` not `var s []T`** — nil slices serialize to JSON `null`.
- **`slices.SortFunc`** not `sort.Slice`. **`t.Context()`** not `context.Background()` in tests.
- **`_, _ = fmt.Fprintf(...)`** — satisfy errcheck in output code.

### Output

All output through `*output.Printer`. Never `fmt.Printf` in commands.

- `p.Info()`, `p.Success()` — suppressed by `--quiet`
- `p.Warn()`, `p.Debug()` — stderr only
- `p.PrintTable()`, `p.PrintJSON()` — always print, never suppressed
- `fmt.Fprintln(p.Out, ...)` — for primary output that must always appear
- Never `cmd.OutOrStdout()` — use `p.Out`

### Error handling

1. API errors → typed (`api.NotFoundError`, `api.PermissionError`)
2. Commands → `tcerrors.UserError` with suggestions via `tcerrors.WithSuggestion(msg, hint)`
3. Root `Execute()` prints `Error: <msg>\nHint: <suggestion>`

Error strings: lowercase, no trailing punctuation. Wrap with `%w`, not bare `return err`.

### Comments

One-line doc comments on funcs. No multi-line restate-the-code text. Inline comments only for non-obvious things (magic numbers, OS quirks, why-not-the-obvious-approach).

## Tests

All new features and bug fixes must include tests. We have a solid integration test setup with testcontainers that spins up a real TeamCity server — please use it. If your change touches API behavior or user-facing commands, an integration test is expected, not just unit tests.

- Prefer testcontainers integration tests over mocks for `api/` behavior.
- Every new command gets an acceptance test in `acceptance/testdata/<noun>/`.
- `require` for setup, `assert` for assertions, `t.Parallel()` where safe.
- `internal/cmdtest/`: `SetupMockClient`, `RunCmdWithFactory`, `RunCmdWithFactoryExpectErr`.

### Test conventions

- **Test env vars**: `t.Setenv(k, v)` only; use `t.Setenv(k, "")` to clear. Never `os.Unsetenv` — it doesn't restore.
- **Test cwd**: small `chdir(t, dir)` helper using `t.Cleanup` to restore. Don't return defer functions.
- **Test surface split**: unit tests cover internal helpers (parsers, cascades, etc.); acceptance scripts (`acceptance/testdata/<sub>/*.txtar`) cover the user-facing binary surface. Don't duplicate — if a `.txtar` asserts `--clear` removes a file, no parallel unit test for the same.
- **Test isolation**: `cmdtest.NewTestServer` calls `Factory.SkipLinkLookup()` so unit tests don't pick up the host's `teamcity.toml`. Pattern this for any future per-cwd config you add.

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

### Writing tests

Each `.txtar` file is a self-contained test script. Key patterns:

```
[!has_token] skip 'requires authentication token'
exec teamcity run list --no-input
stdout '.'
! stderr 'Error'
extract '"id":\s*(\d+)' BUILD_ID
```

Custom commands: `extract`, `wait_for_agent`, `stdout2env`, `env2upper`, `sleep`.
Conditions: `[has_token]`, `[guest]`.

### How they run in CI

Acceptance tests are embedded in the goreleaser build pipeline as a **post-build hook** (`.goreleaser.yaml`). They run automatically after building the CLI binary for the native platform:

- **Snapshot builds** (every push): guest-auth tests — no token needed
- **Release builds** (tagged): token-auth tests using `TEAMCITY_TOKEN` secret — failures block publishing

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

### Test environment

- **Server**: `cli.teamcity.com` (TeamCity Cloud, configurable via `TC_ACCEPTANCE_HOST`)
- **Sandbox project**: use `Sandbox` for any write operations (param set/delete, token put, run start)
- **Cloud agents**: ephemeral — tests that need agents must start a build, wait for assignment, then clean up
- **Isolation**: each test gets its own `HOME` directory, no cross-test state leakage

## Linting

Run `just lint` before pushing. The CI lint job uses `golangci-lint` with
`.golangci.yml` (includes `gocritic`, `misspell`, among others).

Watch for:
- `gocritic/ifElseChain` — rewrite to `switch`
- `misspell` — US locale (canceled, color)
- `errcheck` — excluded in test files only

## Before pushing — checklist

Run, in order:

1. `go test ./...`
2. `just lint` (golangci-lint + `go fmt` + `go fix`)
3. `git status` after lint — `go fmt ./...` may touch unrelated files (e.g. `internal/gallery/`); revert those with `git checkout -- <path>` so they don't bleed into your PR
4. `go test -tags=acceptance ./acceptance/...` if you touched acceptance scripts

## When you change user-facing behavior

Update all three:
1. `docs/topics/` — Writerside topics + GIF if needed
2. `skills/teamcity-cli/` — SKILL.md + references/commands.md + references/workflows.md
3. `README.md` — commands table

Grep the flag/command name across all three before closing the PR.

## Documentation

The canonical documentation lives in [JetBrains/teamcity-documentation](https://github.com/JetBrains/teamcity-documentation) and is published at [jb.gg/tc/docs](https://jb.gg/tc/docs). A local copy is kept in `docs/topics/` for reference and editing convenience.

Use the sync recipes to keep local and upstream docs in sync:

```sh
just docs-pull              # fetch latest from teamcity-documentation
just docs-push              # open a PR to teamcity-documentation with local changes
just docs-generate          # regenerate the CLI command reference table
```

**GIFs:** Terminal recordings (in `docs/images/`) illustrate key workflows. If your change visibly alters CLI output for an existing GIF, re-record it. Use [vhs](https://github.com/charmbracelet/vhs) with tape files in `docs/tapes/`. Always set `TEAMCITY_NO_UPDATE "1"` in tapes. Use `cli.teamcity.com` + `TEAMCITY_GUEST "1"` for public demos, `buildserver.labs.intellij.net` for richer dependency trees.

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

Push your branch and open a PR against `main`. The [PR template](.github/PULL_REQUEST_TEMPLATE.md) will guide you through describing the change — fill in every section it defines.

## AI-assisted contributions

We're fine with AI tools — Junie, Claude Code, Copilot, whatever helps you move faster. But you must understand the code you're submitting. `teamcity` is a tool where we prioritize security and reliability. PRs with AI-generated code that the author can't explain or defend during review will not be merged.

## CI pipeline

Use `https://cli.teamcity.com` (guest auth) when debugging this project's own pipeline — not GitHub Actions.

```sh
TEAMCITY_URL=https://cli.teamcity.com teamcity run list --status failure
```

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

### Troubleshooting Chocolatey

Chocolatey pushes can fail with `503 Service Unavailable` or other transient errors. When this happens, upload the package manually:

1. Check out the release tag and build the package locally:
   ```sh
   git checkout v0.9.0
   ```

2. Download the existing `.nupkg` for the previous version to use as a template:
   ```sh
   curl -L -o old.nupkg 'https://community.chocolatey.org/api/v2/package/TeamCityCLI/<previous-version>'
   unzip old.nupkg -d old/
   ```

3. Create a new package directory with updated `TeamCityCLI.nuspec` (bump `<version>` and `<releaseNotes>` URL) and `tools/chocolateyinstall.ps1` (update the download URL and `checksum64` from `checksums.txt` on the GitHub release page).

4. Pack and push:
   ```sh
   choco apikey --key <YOUR_API_KEY> --source https://push.chocolatey.org/
   choco pack
   choco push TeamCityCLI.<version>.nupkg --source https://push.chocolatey.org/
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
