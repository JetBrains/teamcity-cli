# Concept Mappings: CI Systems to TeamCity

## Contents

- GitHub Actions to TeamCity Pipeline YAML (concepts, actions, runners)
- Fixing a stub
- Bamboo Specs to TeamCity Pipeline YAML (concepts, tasks, variables)
- Bamboo Specs that aren't converted

## GitHub Actions to TeamCity Pipeline YAML

| GitHub Actions | TeamCity | Notes |
|---|---|---|
| `jobs.<id>` | `jobs.<id>` | IDs must use `_` not `-` |
| `steps[].run` | `steps[].script-content` | Shell commands transfer verbatim |
| `steps[].uses: action` | Depends on action | See action mapping below |
| `needs: [job1]` | `dependencies: [job1]` | |
| `runs-on: ubuntu-latest` | `runs-on: Linux-Large` | See runner mapping below |
| `env.KEY: val` | `parameters: env.KEY: val` | |
| `secrets.X` | `%X%` | Add `X: "credentialsJSON:<uuid>"` under `secrets:`; create via `teamcity project token put` |
| `strategy.matrix` | Separate jobs or `parallelism` | |
| `container: image` | `docker-image:` on steps | |
| `services:` | Docker Compose or step-level | |
| `if: condition` | Branch filter or script logic | |
| `timeout-minutes:` | Build configuration timeout (UI) | No YAML equivalent |
| `continue-on-error: true` | Wrap the command so its exit code is ignored (`cmd || true`) or override the step's failure condition | TC fails on nonzero exit; the UI step-execution policy only controls running *after* earlier failures — it won't ignore this step's own exit code |
| `concurrency: { group: ... }` | "Limit max concurrent jobs" build setting (UI) | No YAML equivalent |
| `outputs:` / `${{ steps.x.outputs.y }}` | `output-parameters:` on producer + `%dep.<job>.<param>%` on consumer | Or write to a shared artifact file |
| `uses: ./.github/workflows/x.yml` (reusable) | Inline OR convert separately + snapshot dependency | Stub created |
| `uses: ./.github/actions/x` (composite) | Inline `steps` OR replace with single shell script | Stub created |
| `on: push/pull_request` | VCS trigger (server-side) | |
| `on: schedule` | Scheduled trigger (server-side) | |
| `on: workflow_dispatch` | Manual trigger / parameterized | Inputs become TC build parameters with prompts |

### Action Mapping

The **Converter emits** column is what `teamcity migrate` writes; **Your follow-up** is what you still have to do by hand.

| Action | Converter emits | Your follow-up |
|---|---|---|
| `actions/checkout` | Removed -- TC VCS checkout is automatic | |
| `actions/cache` | `enable-dependency-cache: true` | |
| `actions/upload-artifact` | `files-publication: [{path: "..."}]` | |
| `actions/download-artifact` | Nothing (simplified out); named downloads get a manual note | Artifacts arrive via the job's `dependencies:` (from `needs:`) -- ensure the upstream job publishes with `share-with-jobs: true`, and add the dependency yourself if the workflow downloaded from a job it didn't `need` |
| `actions/setup-node/java/go/python` | Removed -- pre-installed on TC Cloud agents | Pinned versions surface as manual notes; ensure the agent provides them |
| `gradle/actions/setup-gradle` | Removed -- `./gradlew` runs directly | |
| `docker/login-action` | Comment-only placeholder step | Configure a Docker registry connection in TC project settings -- required for private registries, no login command is generated |
| `docker/build-push-action` | `docker build && docker push` script | |
| `JetBrains/qodana-action` | Commented pointer to native integration | Add the Qodana build feature in TC settings |
| `aws-actions/configure-aws-credentials` | `export AWS_*` env lines | Add `env.AWS_ACCESS_KEY_ID` / `env.AWS_SECRET_ACCESS_KEY` under `secrets:` -- the `env.` prefix exports them as environment variables, which is what the script reads |
| `softprops/action-gh-release` | `gh release create "<tag_name>" --generate-notes` plus any `files:` globs; falls back to `%teamcity.build.branch%` when `tag_name` is unset | Add `env.GH_TOKEN` under `secrets:` -- `gh` reads it from the environment, so the `env.` prefix is required |
| `golangci/golangci-lint-action` | `golangci-lint run <args>` | Assumes the binary on the agent; a pinned `version:` becomes a manual note -- install it yourself |
| `codecov/codecov-action` | `curl -Os https://cli.codecov.io/latest/linux/codecov && chmod +x codecov && ./codecov` | |
| `goreleaser/goreleaser-action` | **Stub** (not in the registry) | Replace with `curl -sSfL https://goreleaser.com/static/run \| bash -s -- release --clean` (needs `GITHUB_TOKEN`) |
| `aquasecurity/trivy-action` | `trivy <scan-type> <image-ref>` | Assumes trivy on the agent; install via the trivy install.sh if missing |
| `github/codeql-action/init`, `/analyze`, `/autobuild` | **Dropped** (listed under Needs review) -- requires GitHub security-events API | Consider the Qodana build feature instead. Other codeql-action subpaths (e.g. `/upload-sarif`) stub like unknown actions |
| Unknown actions | Commented stub with original inputs | Read the action's source, write the equivalent shell |

### Fixing a stub

The converter emits unknown actions as commented TODO steps that preserve the original inputs:

```yaml
- type: script
  name: "custom-deploy"
  script-content: |-
    # TODO: Replace acme/custom-deploy@v2 with equivalent commands
    # Action inputs:
    #   region: eu-west-1
    #   target: prod
    echo 'TODO: implement equivalent of custom-deploy'
```

Read the action's repository -- its `action.yml` shows what it actually runs (most actions are thin CLI wrappers). Replace the step body with the equivalent commands:

```yaml
- type: script
  name: "custom-deploy"
  script-content: aws deploy create-deployment --region eu-west-1 --deployment-group prod
```

Secrets referenced by the original inputs become `%PARAM%` references -- create them with `teamcity project token put`.

### Runner Mapping

| GitHub Actions | TeamCity Cloud |
|---|---|
| `ubuntu-latest` / `ubuntu-24.04` / `ubuntu-22.04` | `Linux-Large` |
| `macos-latest` / `macos-15` / `macos-14` | `Mac-Medium` |
| `windows-latest` / `windows-2022` | `Windows-Medium` |
| Self-hosted labels | `self-hosted` with agent requirements |

Hosted agent names come from the server's pipeline schema (`runs-on` enum: `Linux-Small/Medium/Large/XLarge`, `Mac-Medium`, `Windows-Small/Medium` as of 2026.2). When connected, the CLI derives this mapping from the live schema — check `teamcity pipeline schema` if a name is rejected.

## Bamboo Specs to TeamCity Pipeline YAML

Bamboo Specs YAML lives in `bamboo-specs/*.yml` (or `bamboo.yml` in the repo root). The converter walks `stages → jobs → tasks` and turns each task into a TeamCity step.

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
| `requirements[]` | `runs-on` + agent requirements | First OS-shaped entry picks the runner; non-OS entries become manual notes. With no OS requirement, OS-bound tasks (ms-build, fastlane, xcode, ...) infer it |
| `docker.image` | Docker container settings | Surfaced as manual setup; wrap step or use Docker wrapper feature |
| `triggers[]` (polling, cron, ...) | VCS / scheduled triggers | Manual; configure in TC UI |
| `branches:` | VCS root branch filters | Manual; configure on the VCS root |
| `dependencies:` (top-level plan deps) | Cross-pipeline `dependencies:` or snapshot dependencies | Manual |
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
| `node` | `node <script> <args>` | |
| `command` | Inline `<exe> <args>` | |
| `docker` (build/push/run) | `docker <cmd> ...` | Registry login is not converted or flagged -- add `docker login` or a TC registry connection yourself |
| `inject-variables` | `set -a; . file; set +a` | Manual: review whether to convert to TC parameters |
| `dump-variables` | `env \| sort` | |
| `artifact-download` | Manual artifact-dependency | Surfaced as manual setup |
| `test-parser` / `j_unit` / `nunit-parser` / `mocha` | Remove -- TC has built-in test report import | Manual: confirm report path |
| `ssh` / `scp` | Inline `ssh`/`scp` script | Manual: upload SSH key with `teamcity project ssh upload` |
| `ms-build` / `ms-test` / `visual-studio` / `nunit-runner` | Inline equivalent commands | |
| `fastlane` | `fastlane <lane>` | |
| `unlock-keychain` | `security unlock-keychain ...` | Manual: store password as TC token |
| `repository-tag` / `repository-branch` / `repository-commit` / `repository-push` | Inline `git` commands | Push credentials are not flagged -- ensure the agent has them |
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
| Multi-document specs (`---`-separated) | The converter handles the first `plan:` document and flags the rest under Needs review -- split remaining documents into separate files and re-run `teamcity migrate` |
| `repositories:` block (project-level VCS declarations) | Run `teamcity project vcs create` for each repo before pipeline creation |
| `other:` block (`concurrent-build-plugin`, `clean-working-dir`, ...) | Configure cleanup/concurrency in TC build settings UI |
| `linked-repositories:` | Manual: create TC VCS roots and reference by ID |
| Stages/jobs whose names collide after sanitizing | The converter de-duplicates job IDs with `_2` suffixes, but `dependencies:` referencing the duplicated name resolve to the *first* job -- verify the generated `dependencies:` blocks |

