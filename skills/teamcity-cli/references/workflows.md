# Common Workflows

## Inspecting a Build from a TeamCity URL

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

## Investigating a Build Failure

When a build has **FAILURE** status, proactively suggest: `teamcity run log <id> --failed` (failure summary), `teamcity run tests <id> --failed` (failed tests), `teamcity run changes <id>` (triggering changes).

For **composite/matrix builds** (snapshot dependencies, no agent), find failed children with `teamcity run list --status failure` and appropriate filters.

1. **Find the failed build:**
   ```bash
   teamcity run list --status failure -n 10
   ```

2. **View build details:**
   ```bash
   teamcity run view <run-id>
   ```

3. **Check the build log:**
   ```bash
   teamcity run log <run-id> --raw
   ```

   Always use `--raw` to avoid interactive terminal formatting. Dump the output to a temp file to re-read it as needed.

   For failed steps only:
   ```bash
   teamcity run log <run-id> --failed
   ```

4. **View test results:**
   ```bash
   teamcity run tests <run-id>
   ```

   For failed tests only:
   ```bash
   teamcity run tests <run-id> --failed
   ```

5. **See what changes triggered the build:**
   ```bash
   teamcity run changes <run-id>
   ```

## Starting and Monitoring Builds

> **Always use `--watch`** when starting builds to wait until the build finishes before proceeding.
> **Always verify the branch name** — do not guess. Check with `git branch` or `teamcity run list --job <job-id>` to see valid branches.

**Start a build:**
```bash
teamcity run start <job-id> --watch
```

**Start with specific branch:**
```bash
teamcity run start <job-id> --branch feature/my-branch --watch
```

**Start with parameters:**
```bash
teamcity run start <job-id> -P "param1=value1" -P "param2=value2"
```

**Start with env vars and system properties:**
```bash
teamcity run start <job-id> -P version=1.0 -S build.number=123 -E CI=true
```

**Start and watch:**
```bash
teamcity run start <job-id> --watch
teamcity run start <job-id> --watch --timeout 30m
```

**Start with comment and tags:**
```bash
teamcity run start <job-id> --comment "Release build" --tag release --tag v1.0
```

**Start with clean checkout and rebuild deps:**
```bash
teamcity run start <job-id> --clean --rebuild-deps --top
```

**Dry run (see what would be triggered):**
```bash
teamcity run start <job-id> --dry-run
```

**Watch an existing build:**
```bash
teamcity run watch <run-id>
```

**Stream logs while watching:**
```bash
teamcity run watch <run-id> --logs
```

**Watch with timeout:**
```bash
teamcity run watch <run-id> --timeout 30m --quiet
```

**Wait for completion and get JSON result (for scripting):**
```bash
teamcity run start <job-id> --watch --json
teamcity run watch <run-id> --json
```

## Personal Builds (Local Changes)

> **Kotlin DSL caveat:** `--local-changes` does **not** include changes to Kotlin DSL (`.teamcity/`). Always push Kotlin DSL changes to the remote before running the build.

**Run build with uncommitted git changes:**
```bash
teamcity run start <job-id> --local-changes
```

**Run build from a patch file:**
```bash
teamcity run start <job-id> --local-changes changes.patch
```

**Personal build with specific branch:**
```bash
teamcity run start <job-id> --personal --branch my-feature --watch
```

**Skip auto-push:**
```bash
teamcity run start <job-id> --local-changes --no-push
```

## Finding Jobs and Projects

**List all projects:**
```bash
teamcity project list
```

**List sub-projects:**
```bash
teamcity project list --parent <project-id>
```

**List jobs in a project:**
```bash
teamcity job list --project <project-id>
```

**View job details:**
```bash
teamcity job view <job-id>
```

**Search for a job by name:**
```bash
teamcity job list --json | jq '.[] | select(.name | contains("deploy"))'
```

## Working with Build Artifacts

**List artifacts from a build:**
```bash
teamcity run artifacts <run-id>
```

**List artifacts from latest build of a job:**
```bash
teamcity run artifacts --job <job-id>
```

**Download all artifacts:**
```bash
teamcity run download <run-id>
```

**Download to specific directory:**
```bash
teamcity run download <run-id> -o ./artifacts
```

**Download a subdirectory:**
```bash
teamcity run download <run-id> --path build/assets
```

**Download specific artifact pattern:**
```bash
teamcity run download <run-id> --artifact "*.jar"
```

**Combine path and pattern:**
```bash
teamcity run download <run-id> --path build/assets -a "*.js"
```

## Build Metadata

**Pin a build (prevent cleanup):**
```bash
teamcity run pin <run-id> --comment "Release candidate"
```

**Unpin a build:**
```bash
teamcity run unpin <run-id>
```

**Tag a build:**
```bash
teamcity run tag <run-id> deployed production
```

**Remove tags:**
```bash
teamcity run untag <run-id> deployed
```

**Add a comment:**
```bash
teamcity run comment <run-id> "Verified by QA"
```

**View existing comment:**
```bash
teamcity run comment <run-id>
```

**Delete a comment:**
```bash
teamcity run comment <run-id> --delete
```

## Managing the Build Queue

**View queued builds:**
```bash
teamcity queue list
```

**Filter queue by job:**
```bash
teamcity queue list --job <job-id>
```

**Move a build to top of queue:**
```bash
teamcity queue top <run-id>
```

**Remove from queue:**
```bash
teamcity queue remove <run-id>
```

**Approve a build waiting for approval:**
```bash
teamcity queue approve <run-id>
```

## Managing Job and Project Parameters

**List job parameters:**
```bash
teamcity job param list <job-id>
```

**Set a parameter:**
```bash
teamcity job param set <job-id> MY_PARAM "my value"
```

**Set a secure parameter:**
```bash
teamcity job param set <job-id> SECRET_KEY "****" --secure
```

**Get a parameter:**
```bash
teamcity job param get <job-id> MY_PARAM
```

**Delete a parameter:**
```bash
teamcity job param delete <job-id> MY_PARAM
```

Project parameters work the same way with `teamcity project param`.

## Validating Kotlin DSL Locally

**Always use `teamcity project settings validate`** to verify Kotlin DSL — never generic `mvn compile`.

Under the hood it runs `mvn teamcity-configs:generate` (or `./mvnw` when available) inside the `.teamcity/` directory, which is the only correct DSL validation step. Generic Maven commands like `mvn compile` do **not** validate TeamCity DSL and will give misleading results.

```bash
# Preferred — auto-detects .teamcity dir and Maven wrapper
teamcity project settings validate

# Explicit path
teamcity project settings validate ./path/to/.teamcity

# Show full Maven output for debugging
teamcity project settings validate --verbose
```

If you need the raw Maven command (e.g., in CI without the CLI installed):
```bash
./mvnw teamcity-configs:generate -f .teamcity/pom.xml   # prefer wrapper
mvn teamcity-configs:generate -f .teamcity/pom.xml       # fallback
```

## VCS Roots

**List VCS roots in a project:**
```bash
teamcity project vcs list --project <project-id>
```

**View VCS root details:**
```bash
teamcity project vcs view <vcs-root-id>
```

**Open browser to create a VCS root:**
```bash
teamcity project vcs create --project <project-id>
```

**Delete a VCS root:**
```bash
teamcity project vcs delete <vcs-root-id>
teamcity project vcs delete <vcs-root-id> --force   # skip confirmation
```

## Project Settings (Export & Status)

**Check versioned settings sync status (requires server connection):**
```bash
teamcity project settings status <project-id>
```

**Export project settings as Kotlin DSL:**
```bash
teamcity project settings export <project-id>
```

**Export as XML:**
```bash
teamcity project settings export <project-id> --xml -o settings.zip
```

## Secure Tokens

**Store a secret and get a token reference:**
```bash
teamcity project token put <project-id> "my-secret-password"
```

**Store from stdin (for piping):**
```bash
echo -n "my-secret" | teamcity project token put <project-id> --stdin
```

**Retrieve a token value (requires System Admin):**
```bash
teamcity project token get <project-id> "credentialsJSON:abc123..."
```

## Managing Agents

**List all agents:**
```bash
teamcity agent list
```

**List connected agents only:**
```bash
teamcity agent list --connected
```

**Filter agents by pool:**
```bash
teamcity agent list --pool Default
```

**View agent details:**
```bash
teamcity agent view <agent-id>
```

**See what jobs an agent can run:**
```bash
teamcity agent jobs <agent-id>
```

**See why jobs are incompatible with an agent:**
```bash
teamcity agent jobs <agent-id> --incompatible
```

**Enable/disable an agent:**
```bash
teamcity agent enable <agent-id>
teamcity agent disable <agent-id>
```

**Authorize/deauthorize an agent:**
```bash
teamcity agent authorize <agent-id>
teamcity agent deauthorize <agent-id>
```

**Move agent to a different pool:**
```bash
teamcity agent move <agent-id> <pool-id>
```

**Reboot an agent:**
```bash
teamcity agent reboot <agent-id>
```

**Reboot after current build finishes:**
```bash
teamcity agent reboot <agent-id> --after-build
```

## Remote Agent Access

**Open interactive shell on an agent:**
```bash
teamcity agent term <agent-id>
```

**Execute a command on an agent:**
```bash
teamcity agent exec <agent-id> "ls -la"
```

**Execute with timeout:**
```bash
teamcity agent exec <agent-id> --timeout 10m -- long-running-script.sh
```

## Managing Agent Pools

**List all pools:**
```bash
teamcity pool list
```

**View pool details:**
```bash
teamcity pool view <pool-id>
```

**Link a project to a pool:**
```bash
teamcity pool link <pool-id> <project-id>
```

**Unlink a project from a pool:**
```bash
teamcity pool unlink <pool-id> <project-id>
```

## Failure Classification

When a build fails, classify the failure before attempting a fix. The classification determines the fix strategy.

**Decision tree:**

1. **Is the build composite (no agent, has snapshot dependencies)?**
   - Yes → The composite build itself has no logs. Drill into child builds to find the actual failure. Use `teamcity run list --status failure` filtered to the relevant job tree.
2. **Is the failure transient or permanent?**
   - Transient: infrastructure timeouts, agent disconnects, OOM on agent, flaky tests (same code passes on retry). Fix: retry with `teamcity run restart <id>`.
   - Permanent: compilation errors, test failures correlated with code changes, config errors. Fix: change code or config.
3. **Is the failure in code, versioned settings, or server config?**
   - Code: fix in repo, verify with `--local-changes`, push.
   - Versioned settings (Kotlin DSL): fix in repo, validate with `teamcity project settings validate`, push. Cannot use `--local-changes`.
   - Pipeline YAML: fix in repo, validate with `teamcity pipeline validate`, push. Cannot use `--local-changes`.
   - Server config: fix via TeamCity UI or API. Not in repo.

**Default:** treat unknown failures as permanent until proven otherwise.

**Gotchas:**
- Composite builds have empty logs — always drill to child failures first.
- A build can fail with "no compatible agents" — this is server config, not code.
- `--local-changes` does NOT include Kotlin DSL or pipeline YAML stored in repo.

## Build Chain Debugging

TeamCity's snapshot dependency chains are unique — no competitor has this. When a build in a chain fails, the failure cascades upstream, so multiple builds may show as failed.

**Find the root failure:**

```bash
# View the dependency tree for a specific build run (shows statuses)
teamcity run tree <run-id>

# Use --json for programmatic analysis
teamcity run tree <run-id> --json
```

`run tree` shows the actual build runs with their statuses, so you can immediately see which dependency failed. Use `job tree` if you need the job-level (build configuration) dependency structure instead.

**Key principle:** The first failure in the chain (the deepest dependency that failed) is the root cause, not the last. Work bottom-up.

**Steps:**
1. Start from the build the user reported.
2. Run `teamcity run tree <run-id>` to see the full dependency tree with statuses.
3. Find the deepest build in the tree that has a failure status (not just "Snapshot dependency build failed").
4. That's your root cause. Investigate its logs: `teamcity run log <id> --failed --raw`

**Gotchas:**
- Builds that fail only because a dependency failed show "Snapshot dependency build failed" — skip these and go deeper.
- Restarting the top-level build won't help if the root child is still broken.
- Use `run tree` (shows actual builds with statuses) for debugging failures. Use `job tree` (shows build configuration structure) for understanding the dependency graph.

## Fixing a Build Failure

End-to-end workflow for diagnosing and fixing a CI failure. Equivalent to GitHub's `gh-fix-ci`.

### Step 1: Find and diagnose

```bash
# Get the failed build details
teamcity run view <run-id>

# Get the failure log (always use --raw, dump to temp file)
teamcity run log <run-id> --failed --raw > /tmp/build-failure.log

# Check failed tests
teamcity run tests <run-id> --failed

# See what changes triggered the build
teamcity run changes <run-id>
```

### Step 2: Classify the failure

Use the [Failure Classification](#failure-classification) decision tree above.

### Step 3: Fix

**For code failures:**
1. Read the relevant source files and understand the error.
2. Make the fix.
3. Verify locally if possible (run tests, compile, lint).
4. Verify on TeamCity without committing:
   ```bash
   teamcity run start <job-id> --local-changes --watch
   ```
5. Once green, commit and push.

**For versioned settings failures (Kotlin DSL):**
1. Fix the DSL code in `.teamcity/`.
2. Validate locally:
   ```bash
   teamcity project settings validate
   ```
3. Push the fix (cannot use `--local-changes` for DSL).

**For pipeline YAML failures:**
- **Server-stored pipelines:** pull → fix → validate → push:
  ```bash
  teamcity pipeline pull <pipeline-id> -o /tmp/pipeline.yml
  # edit /tmp/pipeline.yml
  teamcity pipeline validate /tmp/pipeline.yml
  teamcity pipeline push <pipeline-id> /tmp/pipeline.yml
  ```
- **VCS-stored pipelines** (`.teamcity.yml` in repo): edit the file directly, validate, then commit and push:
  ```bash
  # edit .teamcity.yml
  teamcity pipeline validate .teamcity.yml
  git add .teamcity.yml && git commit -m "fix: ..." && git push
  ```
  (`pull`/`push` commands fail for VCS-backed pipelines — edit the repo file instead.)

**For server config failures:**
1. Identify the misconfiguration from the logs.
2. Fix via TeamCity UI or `teamcity api`.
3. Restart the build: `teamcity run restart <run-id>`

### Guardrails

- Never delete or skip failing tests to make the build green.
- Never disable linting or static analysis steps.
- Never force-push to fix a build.
- If the fix requires changes outside your expertise, document the diagnosis and escalate.

**Gotchas:**
- Always use `--raw` for logs and dump to a temp file — build logs can be very large and lose formatting without `--raw`.
- `--local-changes` does NOT include Kotlin DSL or pipeline YAML stored in repo. Always push DSL changes before running.
- Composite builds have no logs of their own — drill to the child that actually failed.
- If the build fails with a different error after your fix, that's a new failure — re-diagnose from step 1.

## Monitoring Builds Until Green

Loop workflow for watching a build, fixing failures, and retrying. Equivalent to the `babysit-pr` pattern.

### Loop

1. **Start or watch the build:**
   ```bash
   teamcity run start <job-id> --branch <branch> --watch
   # or watch an existing build:
   teamcity run watch <run-id>
   ```

2. **If the build succeeds:** done.

3. **If the build fails:** run the [Fixing a Build Failure](#fixing-a-build-failure) workflow above.

4. **After pushing the fix:**
   - If the job has a VCS trigger, a new build starts automatically. Poll until a build with a higher ID than the failed one appears, then watch it:
     ```bash
     # Poll for a build on the pushed commit:
     teamcity run list --job <job-id> --branch <branch> --revision @head -n 1 --json
     # Repeat until a result appears (or ~30s pass).
     # If no new build appears, start one manually:
     teamcity run start <job-id> --branch <branch> --watch
     ```
   - If no VCS trigger, start a new build manually:
     ```bash
     teamcity run start <job-id> --branch <branch> --watch
     ```

5. **Repeat** from step 2.

### Stop conditions

- **Success:** the build is green.
- **Max attempts reached:** stop after 3 fix attempts. Each attempt must make different changes — if you're repeating the same fix, something deeper is wrong.
- **Unfixable issue:** server config problem, missing agent, infrastructure failure, or a failure outside the scope of code changes.
- **Same failure after fix:** if the exact same error appears after your fix, re-examine the diagnosis — the fix may not have addressed the root cause.

**Gotchas:**
- A VCS trigger fires only when new commits are pushed to a monitored branch. If the job doesn't have a VCS trigger configured, you must start builds manually with `teamcity run start`.
- After pushing, wait a few seconds before listing runs — the trigger needs time to pick up the change.
- Watch for "build already running" — if a build is queued or running for the same branch, watch it instead of starting a new one.

## Test Reliability Analysis

Identify flaky tests by cross-referencing failures across builds. Equivalent to CircleCI's `find_flaky_tests`.

### Identify potentially flaky tests

```bash
# Get failed tests from the current build
teamcity run tests <run-id> --failed --json > /tmp/failed-tests.json

# Check if the same tests failed in recent builds
teamcity run list --job <job-id> --status failure -n 5 --json

# For each recent failed build, get its failed tests
teamcity run tests <other-run-id> --failed --json
```

### Cross-reference with code changes

```bash
# Check what changed between builds
teamcity run changes <run-id>
```

**Flaky test indicators:**
- Test fails intermittently across builds without corresponding code changes.
- Test passes on retry (restart) without any code change.
- Test fails on one agent but passes on another (environment-dependent).

### What to do with flaky tests

1. Document the flaky test: name, frequency, suspected cause.
2. If `teamcity test mute` becomes available, use it to mute the test with a comment explaining why.
3. Otherwise, flag the test in the codebase (e.g., add a skip annotation with a tracking issue).
4. Never silently delete a flaky test — it may be catching real intermittent bugs.

**Gotchas:**
- A test that fails only on certain agents may be environment-dependent, not flaky. Check agent properties with `teamcity agent view <id>`.
- Some test frameworks report different test names on failure vs success (e.g., parameterized tests). Normalize test names before comparing.
- Large test suites may need `--json` output piped through `jq` for efficient filtering.

## Working with Pipelines

Pipelines are YAML-first build configurations. Unlike jobs (build configs) that are configured via UI or Kotlin DSL, pipelines are defined in a `.teamcity.yml` file. Each pipeline is a TeamCity project containing multiple jobs.

**List pipelines:**
```bash
teamcity pipeline list
teamcity pipeline list --project <project-id>
```

**View pipeline details:**
```bash
teamcity pipeline view <pipeline-id>
teamcity pipeline view <pipeline-id> --web   # open in browser
```

**Create a pipeline from YAML:**
```bash
# --vcs-root is required in non-interactive (agent) usage
teamcity pipeline create my-pipeline --project <project-id> --vcs-root <vcs-root-id>

# From a specific file
teamcity pipeline create my-pipeline --project <project-id> --vcs-root <vcs-root-id> --file pipeline.yml
```

**Validate pipeline YAML before pushing:**
```bash
# Validates against server schema (cached locally for 24h)
teamcity pipeline validate

# Validate a specific file
teamcity pipeline validate my-pipeline.yml

# Force re-fetch schema from server
teamcity pipeline validate --refresh-schema
```

**Pull/push pipeline YAML (edit-in-place workflow):**
```bash
# Download current YAML
teamcity pipeline pull <pipeline-id> -o .teamcity.yml

# Edit the file...

# Validate before pushing
teamcity pipeline validate .teamcity.yml

# Upload changes
teamcity pipeline push <pipeline-id> .teamcity.yml
```

**Delete a pipeline:**
```bash
teamcity pipeline delete <pipeline-id>
teamcity pipeline delete <pipeline-id> --force   # skip confirmation
```

**Gotchas:**
- If the pipeline stores YAML in VCS (versioned settings), `pull` and `push` will return an error — edit the YAML directly in the repo instead.
- `pipeline push` does NOT validate — always run `pipeline validate` first.
- `pipeline create` requires `--project` and `--vcs-root` in non-interactive mode — pipelines always belong to a parent project and VCS root.
- The default YAML file is `.teamcity.yml` in the current directory.

## Tips

1. **Use `--json` for programmatic access** - Parse with `jq` for complex queries

1. **Use `teamcity api` as escape hatch** - When a specific command doesn't exist, use raw API access

1. **Environment variables** - Set `TEAMCITY_URL` and `TEAMCITY_TOKEN` for non-interactive use

1. **Open in browser** - Most view commands support `-w` to open in web browser

1. **Auto-detection from DSL** – When working in a project with Kotlin DSL config, the server URL is auto-detected from `.teamcity/pom.xml`

1. **Multiple servers** - Use `TEAMCITY_URL` env var to switch between servers, or `teamcity auth login --server <url>` to add servers

## Troubleshooting

| Symptom                      | Likely Cause              | Action                                                                                  |
|------------------------------|---------------------------|-----------------------------------------------------------------------------------------|
| `401 Unauthorized`           | Invalid or expired token  | Run `teamcity auth status` to check; re-login with `teamcity auth login`                |
| `403 Forbidden`              | Insufficient permissions  | Build config may require different access rights; check with TeamCity admin             |
| `404 Not Found`              | Build deleted or wrong ID | Verify the build ID/URL; the build may have been cleaned up                             |
| Connection refused / timeout | Server unreachable        | Check if TeamCity instance is accessible; verify server URL with `teamcity auth status` |
| `No server configured`       | Missing auth config       | Run `teamcity auth login -s <url>` or set `TEAMCITY_URL` and `TEAMCITY_TOKEN` env vars  |
| `Network access blocked by sandbox` | Sandbox proxy blocking outbound requests | Add the server domain to the sandbox `allowedDomains`, or exclude `teamcity` from sandboxing |
