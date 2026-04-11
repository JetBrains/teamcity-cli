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
| `runs-on: ubuntu-latest` | `runs-on: Ubuntu-24.04-Large` | See runner mapping below |
| `env.KEY: val` | `parameters: env.KEY: val` | |
| `secrets.X` | `%env.X%` + credentialsJSON | Create via `teamcity project token put` |
| `strategy.matrix` | Separate jobs or `parallelism` | |
| `container: image` | `docker-image:` on steps | |
| `services:` | Docker Compose or step-level | |
| `if: condition` | Branch filter or script logic | |
| `timeout-minutes:` | Step timeout | |
| `on: push/pull_request` | VCS trigger (server-side) | |
| `on: schedule` | Scheduled trigger (server-side) | |
| `on: workflow_dispatch` | Manual trigger / parameterized | |

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
| `ubuntu-latest` / `ubuntu-24.04` | `Ubuntu-24.04-Large` |
| `ubuntu-22.04` | `Ubuntu-22.04-Large` |
| `macos-latest` / `macos-15` | `macOS-15-Sequoia-Large-Arm64` |
| `macos-14` | `macOS-14-Sonoma-Large-Arm64` |
| `windows-latest` / `windows-2022` | `Windows-Server-2022-Large` |
| Self-hosted labels | `self-hosted` with agent requirements |

## GitLab CI/CD to TeamCity

From the [official mapping](https://www.jetbrains.com/help/teamcity/mapping-teamcity-concepts-to-other-ci-terms.html):

| GitLab CI/CD | TeamCity |
|---|---|
| Project | Project |
| Pipeline | Build Chain |
| Stage | Build / Composite Build / Matrix Build |
| Job | Build Configuration |
| Job artifact | Build Artifact |
| Branch/MR pipeline | Build Trigger |
| Runner | Build Agent |
| Rules | Build Step Execution Conditions |

Pipeline YAML specifics:

| GitLab CI | TeamCity Pipeline YAML |
|---|---|
| Job (top-level key) | `jobs.<id>` |
| `script:` | `steps[].script-content` |
| `image:` | `docker-image:` on steps |
| `needs:` | `dependencies:` |
| `variables:` | `parameters:` |
| `artifacts: paths:` | `files-publication:` |
| `cache:` | `enable-dependency-cache: true` |
| `tags:` | `runs-on:` agent requirements |
| `before_script:` | Prepend to `script-content` (same step, not separate) |
| `rules:` / `only:` / `except:` | VCS trigger branch filters |
| `parallel: matrix` | Separate jobs per combination (see gotchas) |
| `artifacts: reports: junit:` | TC auto-detects JUnit XML — no config needed |
| `dependencies: []` | Default in TC — jobs don't share artifacts unless configured |
| GitLab Runner vars (`FF_*`, `GIT_SUBMODULE_STRATEGY`) | Drop — these are runner internals, not build logic |
| GitLab predefined vars (`$CI_COMMIT_*`, etc.) | No TC equivalent — remove or replace with TC parameters |
| Vendor-locked deploy jobs (`registry.gitlab.com/*` images) | Skip — rebuild with TC-native tooling if needed |

## Jenkins to TeamCity

From the [official migration guide](https://www.jetbrains.com/help/teamcity/jenkins-to-teamcity-migration-guidelines.html):

| Jenkins | TeamCity |
|---|---|
| Jenkins Master/Node | TeamCity server |
| Executor | TeamCity Agent |
| View or Folder | Project |
| Job/Item/Project | Build Configuration |
| Build | Build Steps |
| Build Triggers | Build Triggers |
| SCM | VCS Root |
| Workspace | Build Checkout Directory |
| Pipeline | Build Chain (via snapshot dependencies) |
| Label | Agent Requirements |

### Jenkins Plugin to TeamCity Feature

| Jenkins Plugin | TeamCity |
|---|---|
| Pipeline | Build configurations + Kotlin DSL |
| Blue Ocean | Build chains |
| Credentials plugin | Password parameters, credentialsJSON, HashiCorp Vault |
| Artifactory plugin | Built-in artifact storage, S3 buckets |
| Git plugin | First-class Git VCS integration |
| Docker plugin | Built-in Docker and Podman support |
| Slack notification | Built-in Slack/email/Teams notifications |
| Kubernetes plugin | Kubernetes cloud profile or external executor |
| JUnit plugin | Native JUnit parsing, test history, flaky test detection |
| HTML publisher | Built-in HTML report publishing |
| Throttle concurrent builds | Queue prioritization, build limits |
| Workspace cleanup | Built-in cleanup rules with retention |
| Build timeout | Flexible timeout options |
| Parameterized trigger | Snapshot/artifact dependencies with parameter passing |

Pipeline YAML specifics:

| Jenkins (Declarative) | TeamCity Pipeline YAML |
|---|---|
| `pipeline { }` | Pipeline YAML file |
| `agent { label 'x' }` | `runs-on:` |
| `agent { docker { image 'x' } }` | `docker-image:` on steps |
| `stages { stage('X') { } }` | `jobs:` (each stage becomes a job) |
| `steps { sh 'cmd' }` | `steps: [{type: script}]` |
| `environment { KEY = 'val' }` | `parameters: env.KEY: val` |
| `credentials('id')` | `%env.X%` + credentialsJSON |
| `tools { maven/gradle }` | Use `./mvnw` or `./gradlew` as script step |
| `archiveArtifacts` | `files-publication:` |
| `junit '**/*.xml'` | Auto-detected by TC |
| `stash / unstash` | `files-publication` + `share-with-jobs` |

## Bamboo to TeamCity

From the [official mapping](https://www.jetbrains.com/help/teamcity/mapping-teamcity-concepts-to-other-ci-terms.html):

| Bamboo | TeamCity |
|---|---|
| Project | Project |
| Plan | Build Chain |
| Stage | Build / Composite Build / Matrix Build |
| Job | Build Configuration |
| Task | Build Step |
| Artifact | Build Artifact |
| Trigger method | Build Trigger |
| Agent | Build Agent |
| Requirement | Agent Requirement |
| Capability | Build Parameter |

## CircleCI to TeamCity Pipeline YAML

| CircleCI | TeamCity Pipeline YAML |
|---|---|
| `jobs.<id>` | `jobs.<id>` |
| `steps[].run` | `steps[].script-content` |
| `steps[].checkout` | Remove -- TC automatic |
| `workflows.jobs[].requires` | `dependencies:` |
| `docker: [{image: x}]` | `docker-image:` on steps |
| `machine: true` | `runs-on:` |
| `persist_to_workspace` | `files-publication` + `share-with-jobs` |
| `store_artifacts` | `files-publication` |
| `store_test_results` | Auto-detected by TC |
| `save_cache / restore_cache` | `enable-dependency-cache: true` |
| `orbs` | Script equivalents or TC features |

## Azure DevOps to TeamCity Pipeline YAML

| Azure DevOps | TeamCity Pipeline YAML |
|---|---|
| `stages[].jobs[]` | `jobs:` (flatten hierarchy) |
| `steps[].script` / `bash` | `steps[].script-content` |
| `steps[].powershell` | `steps[].script-content` with `powershell -Command` wrapper (see note below) |
| `steps[].task: X@N` | Script or TC feature (map per-task) |
| `pool: vmImage` | `runs-on:` (see runner mapping below) |
| `variables:` | `parameters:` |
| `trigger:` / `pr:` | VCS trigger |
| `dependsOn:` | `dependencies:` |

### Runner Mapping

| Azure DevOps `vmImage` | TeamCity Cloud |
|---|---|
| `ubuntu-latest` / `ubuntu-24.04` / `ubuntu-22.04` / `ubuntu-20.04` / `ubuntu-16.04` | `Ubuntu-24.04-Large` |
| `windows-latest` / `windows-2022` / `windows-2019` / `vs2017-win2016` / `win1803` | `Windows-Server-2022-Large` |
| `macOS-latest` / `macOS-15` / `macOS-14` / `macOS-13` | `macOS-15-Sequoia-Large-Arm64` |

### PowerShell Steps

Azure DevOps `powershell:` steps run in PowerShell natively. TC `type: script` on Windows agents runs in `cmd.exe` by default. Wrap PowerShell content explicitly:

```yaml
- type: script
  name: "PowerShell step"
  script-content: |
    powershell -Command {
      # original PowerShell content here
    }
```

## Travis CI to TeamCity Pipeline YAML

| Travis CI | TeamCity Pipeline YAML |
|---|---|
| `script:` | `steps[].script-content` |
| `before_install` / `install` / `before_script` | Prepend steps |
| `after_script` / `after_success` / `after_failure` | Script conditions or notifications |
| `env.matrix` | Separate jobs or `parallelism` |
| `services:` | Docker or agent features |
| `cache:` | `enable-dependency-cache: true` |
| `deploy:` | Deployment steps |

## Concourse to TeamCity

From the [official mapping](https://www.jetbrains.com/help/teamcity/mapping-teamcity-concepts-to-other-ci-terms.html):

| Concourse | TeamCity |
|---|---|
| Pipeline | Build Chain |
| Job | Build Configuration |
| Build | Build |
| Step | Build Step |
| Artifact | Build Artifact |
| Get step | Build Trigger |
| Worker | Build Agent |
| Vars | Build Parameter |
