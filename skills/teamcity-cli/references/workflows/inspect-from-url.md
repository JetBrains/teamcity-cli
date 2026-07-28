# Inspecting a Build from a TeamCity URL

When a user provides a TeamCity URL, parse it and map to `teamcity` commands.

**Format 1: Specific build** — `https://host/buildConfiguration/ConfigId/12345`
```bash
# Extract build ID (last numeric path segment): 12345
teamcity run view 12345
# If failed:
teamcity run log 12345 --failed --raw
teamcity run tests 12345 --failed
```

**Format 2: Build configuration** — `https://host/buildConfiguration/ConfigId`
```bash
# Extract config ID (last non-numeric path segment): ConfigId
teamcity run list --job ConfigId
```

**Format 3: Project** — `https://host/project/ProjectId`
```bash
# Extract project ID: ProjectId
teamcity job list --project ProjectId
```

Strip query params (`?mode=builds`) and fragments (`#all-projects`) before parsing.
