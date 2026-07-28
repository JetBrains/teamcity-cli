# Investigating a Build Failure

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
