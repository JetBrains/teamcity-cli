# Acceptance Tests

End-to-end blackbox tests for the `teamcity` CLI, written as [txtar](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) scripts executed via the `testscript` framework against a live TeamCity server.

See [CONTRIBUTING.md](../CONTRIBUTING.md#acceptance-tests) for how to run, write, and debug tests.

## Environment Variables

| Variable               | Required | Default                    | Description                                       |
|------------------------|----------|----------------------------|---------------------------------------------------|
| `TC_ACCEPTANCE_HOST`   | No       | `https://cli.teamcity.com` | TeamCity server URL                               |
| `TC_ACCEPTANCE_TOKEN`  | No       | —                          | API token (enables authenticated tests)           |
| `TC_ACCEPTANCE_BINARY` | No       | —                          | Path to pre-built binary (e.g. goreleaser output) |
| `TC_ACCEPTANCE_SCRIPT` | No       | —                          | Filter: only run scripts matching this substring  |

## Directory Structure

```
acceptance/
├── acceptance_test.go          # Test runner with custom commands
├── README.md                   # This file
└── testdata/
    ├── agent/                  # Agent commands (incl. cloud agent lifecycle)
    ├── alias/                  # Alias commands
    ├── api/                    # Raw API commands
    ├── auth/                   # Authentication tests
    ├── help/                   # Help output verification
    ├── job/                    # Job (build config) commands
    ├── pool/                   # Pool commands
    ├── project/                # Project commands
    ├── queue/                  # Queue commands
    ├── run/                    # Run (build) commands
    └── skill/                  # Skill install/update/remove
```
