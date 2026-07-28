# Test Reliability Analysis

Identify flaky tests by cross-referencing failures across builds. Equivalent to CircleCI's `find_flaky_tests`.

## Identify potentially flaky tests

```bash
# Start from one build's failures
teamcity run tests <run-id> --failed --json | jq -r '.testOccurrence[].name'

# Then follow a suspect test across the job's builds (the flakiness signal) and
# turn its history into a pass-rate in one line
teamcity run tests --job <job-id> --test "<name>" --json \
  | jq -r '.testOccurrence | "pass \(map(select(.status=="SUCCESS"))|length)/\(length)"'

# Drop --job for a server-wide history of the same test
teamcity run tests --test "<name>" --json
```

## Cross-reference with code changes

```bash
# Check what changed between builds
teamcity run changes <run-id>
```

**Flaky test indicators:**
- Test fails intermittently across builds without corresponding code changes.
- Test passes on retry (restart) without any code change.
- Test fails on one agent but passes on another (environment-dependent).

## What to do with flaky tests

1. Document the flaky test: name, frequency, suspected cause. Use `teamcity run tests --job <id> --test <name>` to quantify frequency from its pass/fail history.
2. If `teamcity test mute` becomes available, use it to mute the test with a comment explaining why (`run tests` is read-only — it does not mute).
3. Otherwise, flag the test in the codebase (e.g., add a skip annotation with a tracking issue).
4. Never silently delete a flaky test — it may be catching real intermittent bugs.

**Gotchas:**
- A test that fails only on certain agents may be environment-dependent, not flaky. Check agent properties with `teamcity agent view <id>`.
- Some test frameworks report different test names on failure vs success (e.g., parameterized tests). Normalize test names before comparing.
- Large test suites may need `--json` output piped through `jq` for efficient filtering.
