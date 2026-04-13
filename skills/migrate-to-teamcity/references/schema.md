# TeamCity Pipeline YAML Quick Reference

## Structure

```yaml
jobs:
  <job_id>:              # Alphanumeric + underscores only
    name: "Display Name"
    runs-on: <agent>     # See agent types below
    parameters:          # Job-scoped env vars
      env.KEY: "value"
    enable-dependency-cache: true    # Replaces manual caching
    dependencies:        # Jobs that must complete first
      - other_job_id
    steps:
      - type: script     # or: gradle, maven, node-js
        name: "Step Name"
        script-content: |
          echo "hello"
    files-publication:   # Artifacts
      - path: "build/**"
        share-with-jobs: true       # Downstream jobs can access
        publish-artifact: true      # Visible in build results

parameters:              # Pipeline-scoped env vars
  env.GLOBAL_KEY: "value"

secrets:                 # Sensitive values
  env.SECRET_NAME: "credentialsJSON:uuid-here"
```

## Step Types

### `type: script`
```yaml
- type: script
  name: "Run tests"
  script-content: npm test
  working-directory: "subdir"     # Optional
  docker-image: "node:20"        # Optional: run in container
```

### `type: gradle`
```yaml
- type: gradle
  name: "Build"
  gradle-params: clean build -x test
```

### `type: maven`
```yaml
- type: maven
  name: "Build"
  goals: clean package -DskipTests
```

### `type: node-js`
```yaml
- type: node-js
  name: "Build"
  shell-script: npm run build
```

## Agent Types (TeamCity Cloud)

| Agent | OS | Arch |
|---|---|---|
| `Ubuntu-24.04-Large` | Linux | x86_64 |
| `Ubuntu-22.04-Large` | Linux | x86_64 |
| `macOS-15-Sequoia-Large-Arm64` | macOS 15 | ARM64 |
| `macOS-14-Sonoma-Large-Arm64` | macOS 14 | ARM64 |
| `Windows-Server-2022-Large` | Windows | x86_64 |

For self-hosted agents:
```yaml
runs-on:
  self-hosted:
    - os-family: Linux
    - arch: aarch64
```

## Dependencies Between Jobs

```yaml
jobs:
  build:
    steps: [...]

  test:
    dependencies:
      - build              # Simple: wait for build to finish
    steps: [...]

  deploy:
    dependencies:
      - build:
          reuse: successful  # Reuse if nothing changed
      - test
    steps: [...]
```

## Files / Artifacts

```yaml
jobs:
  build:
    files-publication:
      - path: "dist/**"
        share-with-jobs: true      # Other jobs can download
        publish-artifact: true     # Show in UI

  deploy:
    dependencies:
      - build                      # Artifacts available automatically
    download-artifacts:            # From external build configs
      - ExternalConfig_ID:
          from: last-successful
          artifact-rules: "*.jar => lib/"
```

## Validating

```bash
teamcity pipeline validate my-pipeline.tc.yml
```

Uses JSON Schema — reports errors with line numbers.
