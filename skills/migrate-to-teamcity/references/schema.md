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

secrets:                 # Sensitive values — value MUST start with "credentialsJSON:"
  env.SECRET_NAME: "credentialsJSON:uuid-here"
```

The schema enforces the `credentialsJSON:<uuid>` form for every secret. Plain string values are rejected — create the credential first with `teamcity project token put`, then reference its UUID here.

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

| Agent | OS |
|---|---|
| `Linux-Small` / `Linux-Medium` / `Linux-Large` / `Linux-XLarge` | Linux |
| `Mac-Medium` | macOS |
| `Windows-Small` / `Windows-Medium` | Windows |

The authoritative list is the `runs-on` enum in the server's pipeline schema; verify with `teamcity pipeline schema` when in doubt.

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
    download-artifacts:            # From external build configs (NOT in this pipeline)
      - ExternalConfig_ID:
          from: last-successful
          artifact-rules: "*.jar => lib/"
```

`download-artifacts:` is for pulling artifacts from a build configuration outside the current pipeline. For artifacts produced by an upstream job in the same pipeline, declare a `dependencies:` link and mark the artifact `share-with-jobs: true` on the producer — TC then makes them available automatically.

## Validating

```bash
teamcity pipeline validate my-pipeline.tc.yml
```

Uses JSON Schema — reports errors with line numbers.

### What `validate` does NOT check

The schema validates structure (jobs/steps shape, types, secret format, runner field types) but not the inner shape of each step type. That means:

- A misspelled key like `script-conent:` will pass schema validation and fail at runtime.
- `type: gradle` with no `gradle-params` may pass schema but produce an empty build.
- Keys that don't exist (`if:`, `interruptible:`, `timeout:`) pass schema but are silently ignored at runtime.

Treat `pipeline validate` as a fast structural check, not proof of correctness. Always run the pipeline to confirm.
