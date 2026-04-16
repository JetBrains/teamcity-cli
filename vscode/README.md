# TeamCity for VS Code

TeamCity CI/CD integration for Visual Studio Code, powered by the [TeamCity CLI](https://github.com/JetBrains/teamcity-cli).

## Features

- **Pipelines** — Browse and manage TeamCity pipelines, drill into jobs
- **Runs** — View build history, logs, test results, trigger and cancel builds
- **Queue** — Monitor queued builds, approve, reorder
- **Agents** — View agent status, open remote terminals, enable/disable
- **Status Bar** — Current branch build status at a glance with notifications

## Getting Started

1. Install the extension
2. Click the TeamCity icon in the Activity Bar
3. Click "Login to TeamCity" (or "Login as Guest" for public projects)
4. Browse your pipelines and builds

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `teamcity.defaultServer` | `https://cli.teamcity.com` | Default server URL |
| `teamcity.cliPath` | (auto-download) | Custom `teamcity` binary path |
| `teamcity.pollInterval` | `30` | Status bar poll interval (seconds) |
| `teamcity.autoRefresh` | `true` | Auto-refresh tree views |

## License

Apache-2.0
