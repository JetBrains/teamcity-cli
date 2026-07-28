# Starting, Monitoring & Personal Builds

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

**Run build with local git changes:**
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
