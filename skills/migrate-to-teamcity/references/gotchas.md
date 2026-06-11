# Sharp Edges, Troubleshooting, and Manual Setup

## What TeamCity handles natively

These source CI steps are removed because TC does them automatically:

| Source step | Why removed |
|---|---|
| `actions/checkout`, `steps[].checkout` | TC checks out the repo before any step |
| `actions/cache`, `save_cache/restore_cache` | `enable-dependency-cache: true` on the job |
| `actions/setup-*` (java, go, node, python) | Pre-installed on TC Cloud agents |
| `gradle/actions/setup-gradle` | Not needed -- `./gradlew` runs directly |
| `actions/upload-artifact` | `files-publication:` section on the job |
| `actions/download-artifact` | Job `dependencies:` with `share-with-jobs: true` |

## Workflows/jobs to skip (not portable)

Some CI jobs depend on platform-specific infrastructure and cannot be meaningfully migrated:

| Pattern | Why skip |
|---|---|
| CodeQL (`github/codeql-action`) | Requires GitHub security-events API and CodeQL cloud infrastructure |
| Dependabot | GitHub-native dependency update service |
| GitHub Pages deploy (`actions/deploy-pages`) | GitHub-specific hosting; use TC artifact publishing or separate deploy |
| GitHub release creation (`on: release`) | The trigger is GitHub-specific; use tag-based VCS trigger in TC instead |
| Bamboo deployment plans (`bamboo-specs/deployment.yml`) | No TC pipeline equivalent; model as a separate pipeline triggered on build success |
| Bamboo plan permissions (`plan-permissions:`) | Configure project roles in TC Administration → Roles |
| Bamboo notifications block | Configure as TC notification rules per user/project |

When `teamcity migrate` generates stubs for these, delete them rather than trying to fill in the stubs.

## Patterns without YAML equivalents

These need post-migration manual configuration in the TC UI; the YAML alone won't carry them:

| Pattern | Source | TC handling |
|---|---|---|
| Reusable workflows (`uses: ./.github/workflows/x.yml`) | GHA | Inline the called workflow OR convert it separately and use snapshot dependencies |
| Composite actions (`uses: ./.github/actions/x`) | GHA | Inline the action's `steps` OR replace with a single shell script |
| Step outputs (`steps.X.outputs.Y`, `set-output`) | GHA | Use TC `output-parameters:` on the producer job + `%dep.<jobId>.<param>%` on the consumer; or write to a shared file artifact |
| `continue-on-error: true` | GHA | TC step "Execution policy" → "Even if some build steps have failed" (set in UI) |
| `concurrency:` group | GHA | Configure "Limit max concurrent jobs" in build configuration settings |
| `timeout-minutes:` | GHA | Step settings → "Build failure conditions" → execution timeout (UI) |
| Strategy `fail-fast: false` | GHA | Per-job behavior; configure failure conditions in TC build settings |
| `workflow_dispatch` inputs | GHA | TC build parameters with prompt; configure per-pipeline in UI |
| Bamboo `final-tasks:` | Bamboo | Step execution policy "Even if some build steps have failed" (UI) |
| Bamboo `stages[].manual: true` | Bamboo | Manual trigger on the downstream pipeline (UI) |
| Bamboo `triggers:` (polling, cron, build-success) | Bamboo | VCS trigger / scheduled trigger in TC project settings |
| Bamboo `branches:` policy | Bamboo | VCS root branch filter |
| Bamboo plan dependencies (top-level `dependencies:`) | Bamboo | Cross-pipeline `dependencies:` in TC YAML or snapshot dependencies in UI |

## Expanding matrix strategies

TC has no native matrix. Expand each matrix combination into a separate job. The key decision is how to pin the language/tool version:

**Use `docker-image` when the job only needs one toolchain.** This is the cleanest approach for language version matrices (Go, Node, Python, etc.):

```yaml
test_go_1_21:
  name: "test (Go 1.21)"
  runs-on: Linux-Large
  steps:
    - type: script
      docker-image: "golang:1.21"
      script-content: go test -v -race ./...
```

**Install via script when the job needs multiple toolchains.** If a job needs e.g. both a specific Go version AND npm (which isn't in the `golang:` image), run on the agent and install the missing tool:

```yaml
build_oldstable:
  name: "build (oldstable)"
  runs-on: Linux-Large
  steps:
    - type: script
      name: "Install Go 1.23"
      script-content: |
        curl -fsSL "https://go.dev/dl/go1.23.8.linux-amd64.tar.gz" -o /tmp/go.tar.gz
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    - type: script
      script-content: npm install -g some-tool && go test ./...
```

**Use the agent default for `stable`/`latest`.** TC Cloud agents have current versions of Go, Node, Java, and Python pre-installed. Only install explicitly when you need a non-default version.

**Naming convention:** use `<job>_<variant>` IDs — e.g. `test_1_21`, `build_stable`. Job IDs must use `_` not `-`.

**When to simplify instead of expanding.** Large matrices (>6 combinations) produce unwieldy TC pipelines. Pick a representative subset:
- Keep the latest + oldest supported language versions (drop middle versions)
- Keep Linux as the primary OS; add macOS/Windows only if the project has platform-specific code
- For test-tag/flag matrices, keep the default (no flags) + the most important variant (e.g. `-race`)
- Document what was dropped and why in the manual setup notes

## Sharp edges

- **`type: gradle` / `type: maven` use the agent's tool version**, not the project's. Always use `type: script` with `./gradlew` or `./mvnw`. TC native runners are only safe when you control the agent's tool installation.
- **Schema valid does not mean pipeline works.** YAML can pass validation and fail at runtime. Always run.
- **`working-directory` scope differs.** In GH Actions it's relative to repo root. In TC it's relative to the checkout directory (usually the same, but verify).
- **Secrets don't migrate.** Every `${{ secrets.X }}` needs a `credentialsJSON:` parameter created server-side.
- **Triggers are server-side.** `on: push`, `on: pull_request`, `on: schedule` have no YAML equivalent in TC pipelines. Configure in TC UI after pipeline creation.
- **Conditional jobs don't translate directly.** `if: github.ref == 'refs/heads/main'` needs a branch filter on the VCS trigger, not a YAML-level condition.
- **VCS root must be created before the pipeline.** `teamcity pipeline create` accepts `--vcs-root <id>`, not a repo URL. Create it first: `teamcity project vcs create --url <repo> --auth anonymous -p <ProjectId>`. The CLI prints the VCS root ID on success.
- **Default branch defaults to `refs/heads/main`.** Many repos still use `master`. Pass `--branch refs/heads/master` to `teamcity project vcs create` if needed. Check the repo's default branch before creating the VCS root.
- **VCS root auth: no OAuth from CLI.** GitHub OAuth connections require browser-based setup in TC UI. When creating VCS roots via API, use anonymous auth for public repos or upload an SSH key (`teamcity project ssh upload`) and use SSH URL (`git@github.com:...`). For private repos, deploy keys work well -- add the public key as a deploy key in the repo, upload the private key to TC.

### Bamboo-specific

- **Stages flatten into job dependencies.** Bamboo's stage→job model becomes a fan-out of TC `dependencies:`: every job in stage N depends on every job in stage N-1. Cross-stage parallelism within a stage is preserved; per-stage gates (manual/final) are not.
- **`${bamboo.*}` predefined vars map; custom vars don't.** Bamboo's predefined names (`build.number`, `repository.branch.name`, etc.) translate to TC equivalents. Custom `${bamboo.MY_VAR}` references map to `%MY_VAR%` — but the TC parameter must exist; create it via project settings or `teamcity project token put`.
- **Variable scopes collapse.** Bamboo has project / plan / job variable scopes. TC pipelines only have pipeline / job / step parameters. Project- and plan-level Bamboo vars need to be merged into the pipeline `parameters:` block manually.
- **`final-tasks:` are not always-run in TC YAML.** TC only knows execution policies per step. After pipeline creation, set "Even if some build steps have failed" on every step that came from `final-tasks:` (UI only).
- **`other:` block is dropped.** Settings like `concurrent-build-plugin`, `clean-working-dir`, `all-other-apps` have no YAML equivalent — configure cleanup/concurrency in TC build settings.
- **Repository declarations don't move with the spec.** Bamboo `repositories:` blocks become TC VCS roots, but the converter doesn't create them. Run `teamcity project vcs create` for each Bamboo-specs `repositories[]` entry before creating the pipeline.
- **Plan keys aren't IDs.** Bamboo `plan.key: SAMP` ≠ TC pipeline ID. The positional `<name>` argument of `teamcity pipeline create <name>` sets the display name, and TeamCity derives the pipeline ID from it.
- **Multi-plan specs need splitting.** A single Bamboo specs file with several `plan:` blocks isn't supported — split into one file per plan first, then run `teamcity migrate`.

## Troubleshooting

| Failure | Cause | Fix |
|---|---|---|
| `Unsupported class file major version 65` | `type: gradle` using agent's old Gradle with newer JDK | Switch to `type: script` + `./gradlew` |
| `command not found: node/go/python` | Tool not on agent PATH | Check agent, or add setup script |
| `permission denied` on script | File not executable | Add `chmod +x` step or use `bash script.sh` |
| Artifact path not found | `files-publication` path doesn't match build output | Check actual output path in build log |
| Snapshot dependency failed | Upstream job failed | Fix upstream first; `deploy` depends on all others |

## Always-manual setup

| Item | How |
|---|---|
| VCS root | `teamcity project vcs list -p <id>` or create in UI |
| Secrets / GHA `${{ secrets.X }}` / Bamboo `*password*` vars | `teamcity project token put <project-id> "<value>"` |
| Triggers (GHA `on:`, Bamboo `triggers:`) | Configure push/PR/schedule in TC project settings |
| Branch filters (GHA `if:`, Bamboo `branches:`) | Add to VCS trigger for conditional jobs |
| Cloud auth (AWS / GCP / Azure) | TC Connection in project settings |
| GHA `concurrency:`, `timeout-minutes:`, `continue-on-error:` | Build configuration settings in TC UI |
| GHA step outputs / Bamboo plan vars | TC `output-parameters:` + cross-job `%dep.X.Y%` references |
| Bamboo `final-tasks:` | Set "Even if some build steps have failed" on each step (UI) |
| Bamboo `stages[].manual: true` | Manual trigger on the downstream pipeline (UI) |
| Bamboo plan permissions | TC project roles in Administration → Roles |
| Bamboo notifications | TC notification rules per user/project |

## Verification checklist

Before declaring migration complete:

- All `.tc.yml` files pass `teamcity pipeline validate`
- Pipelines created on TC server with correct VCS root
- All jobs ran and passed (not just schema-validated)
- No `# TODO` stubs remaining in the YAML
- Secrets created for all `${{ secrets.* }}` references
- User informed of manual setup items (triggers, branch filters)
