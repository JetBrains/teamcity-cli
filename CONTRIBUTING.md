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
just docs         # regenerate CLI docs in README
just install      # install to $GOPATH/bin
```

Run `just` with no arguments to see all available recipes.

## Tests

All new features and bug fixes must include tests. We have a solid integration test setup with testcontainers that spins up a real TeamCity server — please use it. If your change touches API behavior or user-facing commands, an integration test is expected, not just unit tests.

## AI-assisted contributions

We're fine with AI tools — Junie, Claude Code, Copilot, whatever helps you move faster. But you must understand the code you're submitting. `teamcity` is a tool where we prioritize security and reliability. PRs with AI-generated code that the author can't explain or defend during review will not be merged.

## Documentation

Documentation lives in `docs/topics/` and is built with [Writerside](https://www.jetbrains.com/writerside/). The published site is at [jb.gg/tc/docs](https://jb.gg/tc/docs).

When your change adds or modifies commands, flags, or user-facing behavior, update **all** of the following:

| Location               | What to update                                                                   |
|------------------------|----------------------------------------------------------------------------------|
| `docs/topics/`         | Writerside topic files (`.md`) — the primary documentation                       |
| `skills/teamcity-cli/` | AI agent skill — `SKILL.md`, `references/commands.md`, `references/workflows.md` |
| `README.md`            | Commands table in the root readme                                                |

**GIFs:** Terminal recordings (in `docs/images/`) illustrate key workflows. If your change visibly alters CLI output for an existing GIF, re-record it. Use [vhs](https://github.com/charmbracelet/vhs) with tape files in `docs/tapes/`.

**Keep docs in sync:** It's easy to forget one of the locations above. A good check: grep for the flag or command name you changed across `docs/`, `skills/`, and `README.md` to make sure nothing is stale.

## Submit a pull request

Push your branch and open a PR against `main`. The [PR template](.github/PULL_REQUEST_TEMPLATE.md) will guide you through describing the change.

Before submitting, make sure:

- `just lint` passes
- `just unit` passes (at minimum)
- You've manually tested your change

## Release a new version

> This section is for maintainers.

Releases are handled by [goreleaser](https://goreleaser.com/) and publish to Homebrew, Scoop, Chocolatey, Winget, and GitHub Releases.

Dry-run locally:

```sh
just snapshot         # build a local snapshot
just release-dry-run  # full release process without publishing
```

To cut a release, tag and push:

```sh
git tag -a vX.X.X -m "vX.X.X"
git push origin vX.X.X
```
