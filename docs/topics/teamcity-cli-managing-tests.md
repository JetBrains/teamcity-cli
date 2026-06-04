[//]: # (title: Managing Tests)

<show-structure for="chapter" depth="2"/>

While `teamcity run tests` answers "what broke in *this* build?", the `teamcity test` command group treats tests as entities with their own lifecycle across builds. Use it to find currently failing, muted, or investigated tests in a project or job, inspect a test's pass/fail timeline, and manage mutes and investigations from the CLI.

> Cross-build queries require a scope. Pass `--project` or `--job` (a job takes precedence) — the CLI rejects unbounded, server-wide queries.

| Scope          | Command          | Question it answers                          |
|----------------|------------------|----------------------------------------------|
| Single build   | `teamcity run tests` | "What broke in this build?"              |
| Across builds  | `teamcity test`  | "Is this test reliable? Mute it. Assign it." |

## Listing tests

List the tests currently failing across all builds of a project or job:

```Shell
teamcity test list --project MyProject
teamcity test list --job MyProject_Build
```

`--failing` is the default. Switch the filter with the mutually exclusive `--muted` or `--investigated`:

```Shell
teamcity test list --project MyProject --muted
teamcity test list --project MyProject --investigated
```

The table shows `TEST`, `JOB`, and `SINCE` (when the test last entered this state, with its build number). Use `--json` for the raw occurrence array, and the usual list flags (`--limit`, `--plain`, `--no-header`):

```Shell
teamcity test list --project MyProject --json
teamcity test list --job MyProject_Build --limit 20 --plain
```

## Test history

Show a test's pass/fail timeline across builds:

```Shell
teamcity test history com.example.FooTest.shouldWork --project MyProject
teamcity test history com.example.FooTest.shouldWork --job MyProject_Build
```

The table lists `BUILD`, `STATUS`, `DURATION`, `BRANCH`, and `WHEN`, with a footer summarizing the pass rate and average duration over non-ignored runs:

```
Pass rate: 80% (8/10) | Avg duration: 1.2s
```

`--json` emits the **raw** test-occurrence array verbatim — rich enough for an agent to analyze flakiness. Use `--limit` to bound the number of runs (default 50, `0` for all):

```Shell
teamcity test history com.example.FooTest.shouldWork --project MyProject --json
teamcity test history com.example.FooTest.shouldWork --job MyProject_Build --limit 100
```

## Muting and unmuting

Mute a test in a project or job so its failures stop breaking builds:

```Shell
teamcity test mute com.example.FooTest.flaky --project MyProject --reason "flaky, see TC-123"
```

`--until` controls when the mute lifts (default `permanent`):

```Shell
# Lift automatically once the test passes again
teamcity test mute com.example.FooTest.flaky --job MyProject_Build --until fixed

# Lift at a specific date
teamcity test mute com.example.FooTest.flaky --project MyProject --until 2026-12-31
```

Remove a mute:

```Shell
teamcity test unmute com.example.FooTest.flaky --project MyProject
```

> Mute and unmute take effect immediately without a confirmation prompt — they are reversible, matching `teamcity job param set`.

## Investigations

Assign an investigation for a test (state `TAKEN`):

```Shell
teamcity test investigate com.example.FooTest.flaky --project MyProject
teamcity test investigate com.example.FooTest.flaky --job MyProject_Build --assignee jdoe
```

Close the investigation when the test is fixed (default) or given up on:

```Shell
teamcity test resolve com.example.FooTest.flaky --project MyProject
teamcity test resolve com.example.FooTest.flaky --project MyProject --state given-up
```

## Resolving test names

Mutes and investigations target a test by its internal ID. The CLI resolves the name you pass against the scope's tests. If a name matches more than one test, the command prints the candidate list and exits without acting — disambiguate by passing a more specific name.

## JSON output

Every data command supports `--json` for scripting and agent use:

```Shell
teamcity test list --project MyProject --json
teamcity test history com.example.FooTest.shouldWork --project MyProject --json
```
