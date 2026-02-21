# Acceptance Tests

End-to-end blackbox tests for the `teamcity` CLI binary, modeled after [GitHub CLI's acceptance tests](https://github.com/cli/cli/tree/trunk/acceptance).

Tests are written as [txtar](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) scripts and executed via the `testscript` framework. Each `.txtar` file is a self-contained test that runs CLI commands against a real TeamCity server.

## Running Tests

Tests require the `acceptance` build tag and a TeamCity server to test against.

### Against cli.teamcity.com (guest auth, default)

```bash
go test -v -tags=acceptance ./acceptance -timeout 10m
```

### With a goreleaser snapshot binary

```bash
# Build the snapshot first
goreleaser release --snapshot --clean --skip=publish,chocolatey

# Run acceptance tests against the built binary
TC_ACCEPTANCE_BINARY=dist/teamcity_linux_amd64_v1/teamcity \
  go test -v -tags=acceptance ./acceptance -timeout 10m
```

### With just (recommended)

```bash
# Guest auth against cli.teamcity.com
just acceptance

# Build snapshot + test
just acceptance-snapshot

# Test a specific binary
just acceptance-binary ./path/to/teamcity
```

### With authentication

```bash
TC_ACCEPTANCE_HOST=https://your-server.com \
TC_ACCEPTANCE_TOKEN=your-api-token \
  go test -v -tags=acceptance ./acceptance -timeout 10m
```

### Run a specific test script

```bash
TC_ACCEPTANCE_SCRIPT=project-list \
  go test -v -tags=acceptance ./acceptance -timeout 10m
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TC_ACCEPTANCE_HOST` | No | `https://cli.teamcity.com` | TeamCity server URL |
| `TC_ACCEPTANCE_TOKEN` | No | — | API token (enables authenticated tests) |
| `TC_ACCEPTANCE_BINARY` | No | — | Path to pre-built binary (e.g. goreleaser output) |
| `TC_ACCEPTANCE_SCRIPT` | No | — | Filter: only run scripts matching this substring |

When `TC_ACCEPTANCE_TOKEN` is not set, tests run with guest authentication (`TEAMCITY_GUEST=1`). Authenticated-only tests are skipped.

## Writing Tests

Test scripts live in `testdata/<command>/<name>.txtar`. Each script is a sequence of commands using the [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) syntax.

### Quick reference

| Command | Description | Example |
|---------|-------------|---------|
| `exec` | Run a command (expect exit 0) | `exec teamcity project list` |
| `! exec` | Run a command (expect non-zero exit) | `! exec teamcity run list --limit 0` |
| `stdout` | Assert stdout matches regex | `stdout '"id"'` |
| `stderr` | Assert stderr matches regex | `stderr 'Error'` |
| `! stdout` | Assert stdout does NOT match | `! stdout 'error'` |
| `env` | Set an environment variable | `env TEAMCITY_URL=` |
| `skip_if_no_token` | Skip if no auth token set | `skip_if_no_token` |

### Available environment variables in scripts

| Variable | Description |
|----------|-------------|
| `$TC_HOST` | The target TeamCity server URL |
| `$TEAMCITY_URL` | Same as TC_HOST (set for CLI config) |
| `$TEAMCITY_TOKEN` | Auth token (if provided) |
| `$TEAMCITY_GUEST` | Set to "1" when using guest auth |
| `$TC_HAS_TOKEN` | "1" if token available, "0" otherwise |
| `$RANDOM_STRING` | Random hex string for test isolation |
| `$SCRIPT_NAME` | Current script's base name |

### Conditions

Use `[has_token]` or `[guest]` to conditionally run commands:

```
[has_token] exec teamcity run start MyJob --comment "acceptance test"
[has_token] stdout 'triggered'
```

### Example test script

```txtar
# List projects and verify JSON output structure.

exec teamcity project list --limit 2 --json --no-input
stdout '^\['
stdout '"id"'
stdout '"name"'
! stderr 'Error'
```

### Best practices

- Keep scripts self-contained and independent
- Always use `--no-input` to disable interactive prompts
- Use `--limit` on list commands to keep output manageable
- Use `defer` for cleanup in tests that create resources
- Use `$RANDOM_STRING` to avoid naming collisions
- Test both success and error paths
- Prefer regex assertions over exact string matches for resilience

## Directory Structure

```
acceptance/
├── acceptance_test.go          # Test runner with custom commands
├── README.md                   # This file
└── testdata/
    ├── version/                # Binary version checks
    ├── help/                   # Help output verification
    ├── auth/                   # Authentication tests
    ├── project/                # Project commands
    ├── run/                    # Run (build) commands
    ├── job/                    # Job (build config) commands
    ├── agent/                  # Agent commands
    ├── pool/                   # Pool commands
    ├── api/                    # Raw API commands
    └── queue/                  # Queue commands
```

## CI Integration

Acceptance tests run in TeamCity CI after the GoReleaser snapshot build:

1. **GoReleaser** builds the snapshot binary (`goreleaser release --snapshot`)
2. **Acceptance Tests** run the txtar scripts against the built binary using guest auth on `cli.teamcity.com`

This ensures every release candidate is verified end-to-end before publishing.
