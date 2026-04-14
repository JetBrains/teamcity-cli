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

