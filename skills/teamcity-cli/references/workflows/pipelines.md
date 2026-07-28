# Working with Pipelines

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
teamcity pipeline delete <pipeline-id> --yes   # skip confirmation
```

**Gotchas:**
- If the pipeline stores YAML in VCS (versioned settings), `pull` and `push` will return an error — edit the YAML directly in the repo instead.
- `pipeline push` does NOT validate — always run `pipeline validate` first.
- `pipeline create` requires `--project` and `--vcs-root` in non-interactive mode — pipelines always belong to a parent project and VCS root.
- The default YAML file is `.teamcity.yml` in the current directory.
