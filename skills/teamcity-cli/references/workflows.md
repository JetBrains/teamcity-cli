# Common Workflows

## Investigating a Build Failure

1. **Find the failed build:**
   ```bash
   tc run list --status failure -n 10
   ```

2. **View build details:**
   ```bash
   tc run view <run-id>
   ```

3. **Check the build log:**
   ```bash
   tc run log <run-id>
   ```

   For failed steps only:
   ```bash
   tc run log <run-id> --failed
   ```

4. **View test results:**
   ```bash
   tc run tests <run-id>
   ```

   For failed tests only:
   ```bash
   tc run tests <run-id> --failed
   ```

5. **See what changes triggered the build:**
   ```bash
   tc run changes <run-id>
   ```

## Starting and Monitoring Builds

**Start a build:**
```bash
tc run start <job-id>
```

**Start with specific branch:**
```bash
tc run start <job-id> --branch feature/my-branch
```

**Start with parameters:**
```bash
tc run start <job-id> -P "param1=value1" -P "param2=value2"
```

**Start and watch:**
```bash
tc run start <job-id> --watch
```

**Watch an existing build:**
```bash
tc run watch <run-id>
```

## Finding Jobs and Projects

**List all projects:**
```bash
tc project list
```

**List jobs in a project:**
```bash
tc job list --project <project-id>
```

**View job details:**
```bash
tc job view <job-id>
```

**Search for a job by name:**
```bash
tc job list --json | jq '.[] | select(.name | contains("deploy"))'
```

## Managing the Build Queue

**View queued builds:**
```bash
tc queue list
```

**Move a build to top of queue:**
```bash
tc queue top <run-id>
```

**Remove from queue:**
```bash
tc queue remove <run-id>
```

**Approve a build waiting for approval:**
```bash
tc queue approve <run-id>
```

## Working with Build Artifacts

**Download all artifacts:**
```bash
tc run download <run-id>
```

**Download to specific directory:**
```bash
tc run download <run-id> --dir ./artifacts
```

**Download specific artifact:**
```bash
tc run download <run-id> --artifact "report.html"
```

## Build Metadata

**Pin a build (prevent cleanup):**
```bash
tc run pin <run-id> --comment "Release candidate"
```

**Tag a build:**
```bash
tc run tag <run-id> deployed production
```

**Add a comment:**
```bash
tc run comment <run-id> "Verified by QA"
```

## Personal Builds (Local Changes)

**Run build with uncommitted git changes:**
```bash
tc run start <job-id> --local-changes
```

**Run build from a patch file:**
```bash
tc run start <job-id> --local-changes changes.patch
```

**Personal build with specific branch:**
```bash
tc run start <job-id> --personal --branch my-feature --watch
```

## Managing Agents

**List all agents:**
```bash
tc agent list
```

**List connected agents only:**
```bash
tc agent list --connected
```

**View agent details:**
```bash
tc agent view <agent-id>
```

**See what jobs an agent can run:**
```bash
tc agent jobs <agent-id>
```

**See why jobs are incompatible with an agent:**
```bash
tc agent jobs <agent-id> --incompatible
```

**Enable/disable an agent:**
```bash
tc agent enable <agent-id>
tc agent disable <agent-id>
```

**Move agent to a different pool:**
```bash
tc agent move <agent-id> <pool-id>
```

## Managing Agent Pools

**List all pools:**
```bash
tc pool list
```

**View pool details:**
```bash
tc pool view <pool-id>
```

**Link a project to a pool:**
```bash
tc pool link <pool-id> <project-id>
```

**Unlink a project from a pool:**
```bash
tc pool unlink <pool-id> <project-id>
```

## Tips

1. **Use `--json` for programmatic access** - Parse with `jq` for complex queries

2. **Use `tc run log` interactively** - It has built-in search (`/`), navigation (`n`, `N`, `g`, `G`), and filtering (`&pattern`)

3. **Use `tc api` as escape hatch** - When a specific command doesn't exist, use raw API access

4. **Environment variables** - Set `TEAMCITY_URL` and `TEAMCITY_TOKEN` for non-interactive use

5. **Open in browser** - Most view commands support `-w` to open in web browser
