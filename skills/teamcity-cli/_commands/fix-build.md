---
disable-model-invocation: true
allowed-tools:
  - Bash(teamcity *)
  - Bash(git *)
  - Read
  - Edit
  - Write
  - Grep
  - Glob
  - Agent
description: Diagnose and fix a failing TeamCity build. Finds the failure, classifies it, proposes a fix, and verifies it with --local-changes before committing.
---

# /fix-build

Fix a failing TeamCity build.

## Arguments

`$ARGUMENTS` — optional build ID or TeamCity URL. If omitted, finds the most recent failure.

## Workflow

### 1. Find the failing build

```bash
# If a build ID or URL was provided:
teamcity run view <id>

# Otherwise, find the most recent failure:
teamcity run list --status failure -n 1
```

### 2. Diagnose

```bash
teamcity run log <id> --failed --raw > /tmp/build-failure.log
teamcity run tests <id> --failed
teamcity run changes <id>
```

Read `/tmp/build-failure.log` to understand the failure. If this is a composite build (no agent), find the failed child build first — composite builds have empty logs.

For build chains, use `teamcity run tree <run-id>` to see the dependency tree with statuses — the deepest failed dependency is the root cause.

### 3. Classify

Determine if the failure is:
- **Code failure** — compilation error, test failure, lint failure
- **Config failure** — Kotlin DSL or pipeline YAML issue
- **Infrastructure failure** — agent issue, timeout, no compatible agents (cannot fix from code)

### 4. Present diagnosis

Before making any changes, present:
- What failed and why
- Which files need to change
- The proposed fix

Wait for user approval before proceeding.

### 5. Fix and verify

**For code failures:**
1. Make the fix.
2. Run local tests/lint if applicable.
3. Verify on TeamCity:
   ```bash
   teamcity run start <job-id> --local-changes --watch
   ```
4. If green, report success. Do not commit — let the user decide when to commit.

**For Kotlin DSL config failures:**
1. Fix the DSL code in `.teamcity/`.
2. Validate: `teamcity project settings validate`
3. Report the fix. DSL changes cannot use `--local-changes` — the user must push to verify on the server.

**For pipeline YAML failures:**
1. Fix the YAML in `.teamcity.yml` (or pull it first: `teamcity pipeline pull <id> -o .teamcity.yml`).
2. Validate: `teamcity pipeline validate`
3. Push: `teamcity pipeline push <id>` (if server-stored) or commit and push (if VCS-stored).

**For infrastructure failures:**
Report the diagnosis and recommend what the TeamCity admin needs to change. Do not attempt code fixes.

## Guardrails

- Never delete or skip tests.
- Never disable linting or analysis steps.
- Never force-push.
- Make the minimal change needed to fix the failure.
- Do not commit unless the user explicitly asks.
