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

**Start with env vars and system properties:**
```bash
tc run start <job-id> -P version=1.0 -S build.number=123 -E CI=true
```

**Start and watch:**
```bash
tc run start <job-id> --watch
```

**Start with comment and tags:**
```bash
tc run start <job-id> --comment "Release build" --tag release --tag v1.0
```

**Start with clean checkout and rebuild deps:**
```bash
tc run start <job-id> --clean --rebuild-deps --top
```

**Dry run (see what would be triggered):**
```bash
tc run start <job-id> --dry-run
```

**Watch an existing build:**
```bash
tc run watch <run-id>
```

**Stream logs while watching:**
```bash
tc run watch <run-id> --logs
```

**Watch with timeout:**
```bash
tc run watch <run-id> --timeout 30m --quiet
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

**Skip auto-push:**
```bash
tc run start <job-id> --local-changes --no-push
```

## Finding Jobs and Projects

**List all projects:**
```bash
tc project list
```

**List sub-projects:**
```bash
tc project list --parent <project-id>
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

## Working with Build Artifacts

**List artifacts from a build:**
```bash
tc run artifacts <run-id>
```

**List artifacts from latest build of a job:**
```bash
tc run artifacts --job <job-id>
```

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
tc run download <run-id> --artifact "*.jar"
```

## Build Metadata

**Pin a build (prevent cleanup):**
```bash
tc run pin <run-id> --comment "Release candidate"
```

**Unpin a build:**
```bash
tc run unpin <run-id>
```

**Tag a build:**
```bash
tc run tag <run-id> deployed production
```

**Remove tags:**
```bash
tc run untag <run-id> deployed
```

**Add a comment:**
```bash
tc run comment <run-id> "Verified by QA"
```

**View existing comment:**
```bash
tc run comment <run-id>
```

**Delete a comment:**
```bash
tc run comment <run-id> --delete
```

## Managing the Build Queue

**View queued builds:**
```bash
tc queue list
```

**Filter queue by job:**
```bash
tc queue list --job <job-id>
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

## Managing Job and Project Parameters

**List job parameters:**
```bash
tc job param list <job-id>
```

**Set a parameter:**
```bash
tc job param set <job-id> MY_PARAM "my value"
```

**Set a secure parameter:**
```bash
tc job param set <job-id> SECRET_KEY "****" --secure
```

**Get a parameter:**
```bash
tc job param get <job-id> MY_PARAM
```

**Delete a parameter:**
```bash
tc job param delete <job-id> MY_PARAM
```

Project parameters work the same way with `tc project param`.

## Project Settings (Versioned/DSL)

**Validate Kotlin DSL configuration:**
```bash
tc project settings validate
```

**Validate with verbose Maven output:**
```bash
tc project settings validate --verbose
```

**Check versioned settings sync status:**
```bash
tc project settings status <project-id>
```

**Export project settings as Kotlin DSL:**
```bash
tc project settings export <project-id>
```

**Export as XML:**
```bash
tc project settings export <project-id> --xml -o settings.zip
```

## Secure Tokens

**Store a secret and get a token reference:**
```bash
tc project token put <project-id> "my-secret-password"
```

**Store from stdin (for piping):**
```bash
echo -n "my-secret" | tc project token put <project-id> --stdin
```

**Retrieve a token value (requires System Admin):**
```bash
tc project token get <project-id> "credentialsJSON:abc123..."
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

**Filter agents by pool:**
```bash
tc agent list --pool Default
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

**Authorize/deauthorize an agent:**
```bash
tc agent authorize <agent-id>
tc agent deauthorize <agent-id>
```

**Move agent to a different pool:**
```bash
tc agent move <agent-id> <pool-id>
```

**Reboot an agent:**
```bash
tc agent reboot <agent-id>
```

**Reboot after current build finishes:**
```bash
tc agent reboot <agent-id> --after-build
```

## Remote Agent Access

**Open interactive shell on an agent:**
```bash
tc agent term <agent-id>
```

**Execute a command on an agent:**
```bash
tc agent exec <agent-id> "ls -la"
```

**Execute with timeout:**
```bash
tc agent exec <agent-id> --timeout 10m -- long-running-script.sh
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

6. **Auto-detection from DSL** – When working in a project with Kotlin DSL config, the server URL is auto-detected from `.teamcity/pom.xml`

7. **Multiple servers** - Use `TEAMCITY_URL` env var to switch between servers, or `tc auth login --server <url>` to add servers
