# Fixing a Build Failure & Monitoring Until Green

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

Use the [Failure Classification](investigate-failure.md#failure-classification) decision tree.

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
