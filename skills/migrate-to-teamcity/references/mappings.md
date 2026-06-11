# Concept Mappings: CI Systems to TeamCity

Official TeamCity migration docs:
- [Jenkins to TeamCity Migration Guidelines](https://www.jetbrains.com/help/teamcity/jenkins-to-teamcity-migration-guidelines.html)
- [Mapping TeamCity Concepts to Other CI Terms](https://www.jetbrains.com/help/teamcity/mapping-teamcity-concepts-to-other-ci-terms.html)

## General TeamCity Terminology

| Common CI term | TeamCity term |
|---|---|
| Pipeline / Workflow | Build Chain |
| Job / Stage | Build Configuration |
| Build / Run | Build |
| Step / Task | Build Step |
| Runner / Agent / Executor | Build Agent |
| Agent pool / Runner group | Agent Pool |
| Variables / Env vars | Build Parameters |
| Secrets / Credentials | Password-type parameters, credentialsJSON |
| Artifact | Build Artifact |
| Trigger (push, PR, schedule) | Build Trigger |
| Source control / Repository | VCS Root |
| Working directory | Build Checkout Directory |
| Agent label / Tag | Agent Requirement |

## GitHub Actions to TeamCity Pipeline YAML

| GitHub Actions | TeamCity | Notes |
|---|---|---|
| `jobs.<id>` | `jobs.<id>` | IDs must use `_` not `-` |
| `steps[].run` | `steps[].script-content` | Shell commands transfer verbatim |
| `steps[].uses: action` | Depends on action | See action mapping below |
| `needs: [job1]` | `dependencies: [job1]` | |
| `runs-on: ubuntu-latest` | `runs-on: Linux-Large` | See runner mapping below |
| `env.KEY: val` | `parameters: env.KEY: val` | |
| `secrets.X` | `%env.X%` + credentialsJSON | Create via `teamcity project token put` |
| `strategy.matrix` | Separate jobs or `parallelism` | |
| `container: image` | `docker-image:` on steps | |
| `services:` | Docker Compose or step-level | |
| `if: condition` | Branch filter or script logic | |
| `timeout-minutes:` | Build configuration timeout (UI) | No YAML equivalent |
| `continue-on-error: true` | Step "Even if some build steps have failed" policy (UI) | No YAML equivalent |
| `concurrency: { group: ... }` | "Limit max concurrent jobs" build setting (UI) | No YAML equivalent |
| `outputs:` / `${{ steps.x.outputs.y }}` | `output-parameters:` on producer + `%dep.<job>.<param>%` on consumer | Or write to a shared artifact file |
| `uses: ./.github/workflows/x.yml` (reusable) | Inline OR convert separately + snapshot dependency | Stub created |
| `uses: ./.github/actions/x` (composite) | Inline `steps` OR replace with single shell script | Stub created |
| `on: push/pull_request` | VCS trigger (server-side) | |
| `on: schedule` | Scheduled trigger (server-side) | |
| `on: workflow_dispatch` | Manual trigger / parameterized | Inputs become TC build parameters with prompts |

### Action Mapping

| Action | TeamCity |
|---|---|
| `actions/checkout` | Remove -- TC VCS checkout is automatic |
| `actions/cache` | `enable-dependency-cache: true` |
| `actions/upload-artifact` | `files-publication: [{path: "..."}]` |
| `actions/download-artifact` | Job dependencies with `share-with-jobs` |
| `actions/setup-node/java/go/python` | Remove -- pre-installed on TC Cloud agents |
| `gradle/actions/setup-gradle` | Remove -- `./gradlew` runs directly |
| `docker/login-action` | TC Docker registry connection |
| `docker/build-push-action` | `docker build && docker push` script |
| `JetBrains/qodana-action` | TC native Qodana build feature |
| `aws-actions/configure-aws-credentials` | TC AWS Connection in project settings |
| `softprops/action-gh-release` | `gh release create "$TAG" --generate-notes` script |
| `golangci/golangci-lint-action` | `curl -sSfL .../golangci-lint/.../install.sh \| sh -s -- -b $(go env GOPATH)/bin <version>` then `golangci-lint run` |
| `codecov/codecov-action` | `curl -Os https://cli.codecov.io/latest/linux/codecov && chmod +x codecov && ./codecov` |
| `goreleaser/goreleaser-action` | `curl -sSfL https://goreleaser.com/static/run \| bash -s -- release --clean` (needs `GITHUB_TOKEN` secret) |
| `aquasecurity/trivy-action` | `curl -sfL .../trivy/.../install.sh \| sh -s -- -b /usr/local/bin` then `trivy fs .` with matching flags |
| `github/codeql-action/*` | **Skip** -- GitHub-specific, requires GitHub security-events API. Not portable to TC |
| Unknown actions | Commented stub with original inputs |

### Runner Mapping

| GitHub Actions | TeamCity Cloud |
|---|---|
| `ubuntu-latest` / `ubuntu-24.04` / `ubuntu-22.04` | `Linux-Large` |
| `macos-latest` / `macos-15` / `macos-14` | `Mac-Medium` |
| `windows-latest` / `windows-2022` | `Windows-Medium` |
| Self-hosted labels | `self-hosted` with agent requirements |

Hosted agent names come from the server's pipeline schema (`runs-on` enum: `Linux-Small/Medium/Large/XLarge`, `Mac-Medium`, `Windows-Small/Medium` as of 2026.2). When connected, the CLI derives this mapping from the live schema — check `teamcity pipeline schema` if a name is rejected.

## Bamboo Specs to TeamCity Pipeline YAML

Bamboo Specs YAML lives in `bamboo-specs/*.yml` (or `bamboo.yml` in the repo root). The converter walks `stages → jobs → tasks` and turns each task into a TeamCity step. Stage ordering becomes job dependencies: every job in stage N depends on every job in stage N-1.

| Bamboo concept | TeamCity | Notes |
|---|---|---|
| `plan` (project-key, key, name) | Pipeline + project | Project must exist before `pipeline create` |
| `stages[]` ordered list | Job dependencies | Stage N's jobs `dependencies:` all stage N-1 jobs |
| `stages[].manual: true` | Manual approval | Surfaced as manual setup; use TC manual trigger or approval feature |
| `stages[].final: true` | Final cleanup job | Set step execution policy to "Even if some build steps have failed" |
| Top-level job def (e.g. `Build:`) | TC job | Job ID becomes `<Stage>_<Job>` (sanitized) |
| `tasks[]` | `steps[]` | Each task transformed individually; unknowns become TODO stubs |
| `final-tasks[]` | Steps with always-run policy | Surfaced as manual setup; set per-step `Even if some build steps have failed` |
| `artifacts[]` | `files-publication[]` | `shared: true` → `share-with-jobs`; otherwise `publish-artifact` |
| `artifact-subscriptions[]` | Artifact dependencies | Manual: add to pipeline `dependencies:` block |
| `requirements[]` | `runs-on` + agent requirements | First entry maps via runner table; rest become manual notes |
| `docker.image` | Docker container settings | Surfaced as manual setup; wrap step or use Docker wrapper feature |
| `triggers[]` (polling, cron, ...) | VCS / scheduled triggers | Manual; configure in TC UI |
| `branches:` | VCS root branch filters | Manual; configure on the VCS root |
| `variables:` | Pipeline parameters | Lifted to top-level `parameters:` |
| `${bamboo.foo}` references | `%foo%` (TC parameter) | Predefined names map to TC equivalents (see below) |
| `plan-permissions:` | Project roles | Manual; configure in TC Administration → Roles |
| `notifications:` | Notification rules | Manual; configure per project/user |

### Bamboo Task Mapping

| Bamboo task | TeamCity step | Notes |
|---|---|---|
| `script` | `type: script` | Shorthand list and full form (`scripts:`, `interpreter:`) supported |
| `checkout` | Remove -- TC VCS checkout is automatic | |
| `clean` | Remove -- enable "Clean checkout" on VCS root | |
| `maven` / `mvn2` / `mvn3` | `mvn -f <project> <goal>` | JDK and `tests:` flag surface as manual notes |
| `ant` | `ant -f <buildfile> <target>` | |
| `gradle` | `./gradlew <tasks>` | |
| `npm` | `npm <command>` | |
| `node` / `node_unit` | `node <script> <args>` | |
| `command` | Inline `<exe> <args>` | |
| `docker` (build/push/run) | `docker <cmd> ...` | Manual notes for registry credentials |
| `inject-variables` | `set -a; . file; set +a` | Manual: review whether to convert to TC parameters |
| `dump-variables` | `env \| sort` | |
| `artifact-download` | Manual artifact-dependency | Surfaced as manual setup |
| `test-parser` / `j_unit` / `nunit-parser` / `mocha` | Remove -- TC has built-in test report import | Manual: confirm report path |
| `ssh` / `scp` | Inline `ssh`/`scp` script | Manual: upload SSH key with `teamcity project ssh-key upload` |
| `ms-build` / `ms-test` / `visual-studio` / `nunit-runner` | Inline equivalent commands | |
| `fastlane` | `fastlane <lane>` | |
| `unlock-keychain` | `security unlock-keychain ...` | Manual: store password as TC token |
| `repository-tag` / `repository-branch` / `repository-commit` / `repository-push` | Inline `git` commands | Manual: ensure agent has push credentials |
| `aws-code-deploy` | `aws deploy create-deployment ...` | Manual: store AWS credentials as TC tokens |
| `grails` / `gulp` / `grunt` / `bower` | Inline runner invocation | |
| Unknown task | Commented TODO stub with original fields | |

### Bamboo Variable Mapping

`${bamboo.foo}` references in task fields map to TC parameter syntax:

| Bamboo | TeamCity |
|---|---|
| `${bamboo.build.number}` | `%build.number%` |
| `${bamboo.repository.revision.number}` | `%build.vcs.number%` |
| `${bamboo.repository.branch.name}` | `%teamcity.build.branch%` |
| `${bamboo.repository.git.repositoryUrl}` | `%vcsroot.url%` |
| `${bamboo.working.directory}` | `%teamcity.build.checkoutDir%` |
| `${bamboo.tmp.directory}` | `%system.teamcity.build.tempDir%` |
| `${bamboo.buildPlanName}` | `%teamcity.buildConfName%` |
| `${bamboo.planKey}` / `${bamboo.buildKey}` | `%system.teamcity.buildType.id%` |
| `${bamboo.agentId}` | `%teamcity.agent.id%` |
| `${bamboo.build.timeStamp}` | `%build.start.date.timestamp%` |
| `${bamboo.<custom>}` | `%<custom>%` (define in TC project parameters) |
| `${SHELL_VAR}` (no `bamboo.` prefix) | Left untouched (treated as shell expansion) |

### Bamboo Specs that aren't converted

These constructs land in the bamboo-specs directory but the migrate command does not auto-convert them — handle manually:

| Bamboo construct | TeamCity handling |
|---|---|
| `bamboo-specs/deployment.yml` (deployment plans) | Model as a separate pipeline triggered on the build pipeline's success; or use TC deployment build configuration |
| Multi-plan specs (multiple `plan:` blocks in one file) | Split into one file per plan, then re-run `teamcity migrate` |
| `repositories:` block (project-level VCS declarations) | Run `teamcity project vcs create` for each repo before pipeline creation |
| `other:` block (`concurrent-build-plugin`, `clean-working-dir`, ...) | Configure cleanup/concurrency in TC build settings UI |
| `linked-repositories:` | Manual: create TC VCS roots and reference by ID |
| Multiple stages with the same name | Bamboo allows it; TC requires unique job IDs — rename in Bamboo before migration |

