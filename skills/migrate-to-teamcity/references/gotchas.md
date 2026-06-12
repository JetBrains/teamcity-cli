# Sharp Edges, Troubleshooting, and Manual Setup

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

The converter drops the steps it recognizes as non-portable (listed under "Needs review") and emits a no-op placeholder when a job ends up with no steps — delete those placeholders rather than trying to fill them in. Unrecognized variants (e.g. `github/codeql-action/upload-sarif`) become regular TODO stubs instead — decide per stub whether to replace or remove it.

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

**Use the agent default for `stable`/`latest`.** TC Cloud agents have current versions of Go, Node, Java, and Python pre-installed. Only install explicitly when you need a non-default version (e.g. Go's `oldstable` → install the previous minor release).

**Naming convention:** use `<job>_<variant>` IDs — e.g. `test_1_21`, `build_stable`. Job IDs must use `_` not `-`.

**When to simplify instead of expanding.** Large matrices (>6 combinations) produce unwieldy TC pipelines. Pick a representative subset:
- Keep the latest + oldest supported language versions (drop middle versions)
- Keep Linux as the primary OS; add macOS/Windows only if the project has platform-specific code
- For test-tag/flag matrices, keep the default (no flags) + the most important variant (e.g. `-race`)
- Document what was dropped and why in the manual setup notes

## Sharp edges

- **`working-directory` scope differs.** In GH Actions it's relative to repo root. In TC it's relative to the checkout directory (usually the same, but verify).

### Bamboo-specific

- **Variable scopes collapse.** Bamboo has project / plan / job variable scopes. TC pipelines only have pipeline / job / step parameters. Project- and plan-level Bamboo vars need to be merged into the pipeline `parameters:` block manually.
- **Plan keys aren't IDs.** Bamboo `plan.key: SAMP` ≠ TC pipeline ID. The positional `<name>` argument of `teamcity pipeline create <name>` sets the display name, and TeamCity derives the pipeline ID from it.

## Troubleshooting

| Failure | Cause | Fix |
|---|---|---|
| `Unsupported class file major version 65` | `type: gradle` using agent's old Gradle with newer JDK | Switch to `type: script` + `./gradlew` |
| `command not found: node/go/python` | Tool not on agent PATH | Check agent, or add setup script |
| `permission denied` on script | File not executable | Add `chmod +x` step or use `bash script.sh` |
| Artifact path not found | `files-publication` path doesn't match build output | Check actual output path in build log |
| Snapshot dependency failed | Upstream job failed | Fix the deepest failed upstream job first |

## Always-manual setup

| Item | How |
|---|---|
| VCS root | `teamcity project vcs list -p <id>` or create in UI |
| Secrets / GHA `${{ secrets.X }}` / Bamboo `*password*` vars | `teamcity project token put <project-id> "<value>"` |
| Triggers (GHA `on:`, Bamboo `triggers:`) | Configure push/PR/schedule in TC project settings |
| Branch filters (GHA `if:`, Bamboo `branches:`) | Add to VCS trigger for conditional jobs |
| Cloud auth (AWS / GCP / Azure) | TC Connection in project settings |
| GHA `concurrency:`, `timeout-minutes:`, `fail-fast: false` | Build configuration settings in TC UI |
| GHA `continue-on-error: true` | Wrap the command (`cmd || true`) or override the step's failure condition — the UI step policy won't ignore the step's own exit code |
| GHA step outputs / Bamboo plan vars | TC `output-parameters:` + cross-job `%dep.X.Y%` references |
| Bamboo `final-tasks:` | Set "Even if some build steps have failed" on each step (UI) |
| Bamboo `stages[].manual: true` | Manual trigger on the downstream pipeline (UI) |
| Bamboo plan permissions | TC project roles in Administration → Roles |
| Bamboo notifications | TC notification rules per user/project |
