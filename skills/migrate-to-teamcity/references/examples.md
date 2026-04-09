# Migration Examples

## Go Multi-Version Matrix (GitHub Actions → TeamCity)

Tested on cli.teamcity.com (stretchr/testify).

**Result: 11 steps × 2 matrix jobs → 10 jobs, 7 unique steps.** Matrix expanded into separate jobs. Boilerplate removed (checkout ×2, setup-go ×2).

### Before (GitHub Actions)

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go_version: [stable, oldstable]
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with: { go-version: "${{ matrix.go_version }}" }
      - run: npm install -g mdsf-cli
      - run: ./.ci.gogenerate.sh
      - run: ./.ci.gofmt.sh
      - run: ./.ci.readme.fmt.sh
      - run: ./.ci.govet.sh
      - run: go test -v -race ./...
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go_version: ["1.17", "1.18", "1.19", "1.20", "1.21", "1.22", "1.23", "1.24"]
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with: { go-version: "${{ matrix.go_version }}" }
      - run: go test -v -race ./...
```

### After (TeamCity Pipeline YAML)

```yaml
jobs:
  build_stable:
    name: "build (stable)"
    runs-on: Ubuntu-24.04-Large
    steps:
      - type: script
        name: "Install mdsf-cli"
        script-content: npm install -g mdsf-cli
      - type: script
        script-content: ./.ci.gogenerate.sh
      # ... remaining lint/test steps on agent default Go

  build_oldstable:
    name: "build (oldstable)"
    runs-on: Ubuntu-24.04-Large
    steps:
      - type: script
        name: "Install Go oldstable"
        script-content: |
          curl -fsSL "https://go.dev/dl/go1.23.8.linux-amd64.tar.gz" -o /tmp/go.tar.gz
          sudo rm -rf /usr/local/go
          sudo tar -C /usr/local -xzf /tmp/go.tar.gz
      # ... same lint/test steps, now using Go 1.23

  test_1_21:   # repeated for each Go version
    name: "test (1.21)"
    runs-on: Ubuntu-24.04-Large
    steps:
      - type: script
        docker-image: "golang:1.21"
        script-content: go test -v -race ./...
```

### Lessons from this migration (Go matrix)

1. **`docker-image` is the clean path for single-toolchain version matrices.** Each test job pins its Go version via `docker-image: golang:X.Y`. No install scripts needed.
2. **Mixed-toolchain jobs can't use `docker-image`.** The build job needs both Go (specific version) and npm (for mdsf-cli). The `golang:` image doesn't have npm. Solution: run on the agent (which has both Go and Node) and install the non-default Go version via tarball.
3. **`softprops/action-gh-release` → `gh release create`.** The converter stubs this. Replace with `gh release create "$TAG" --generate-notes`. Needs a `GITHUB_TOKEN` secret configured via `teamcity project token put`.
4. **`stable`/`oldstable` are Go-specific aliases.** Map `stable` to the agent's default Go; for `oldstable`, install the previous minor release explicitly.

## Go Library with Docker Images (GitLab CI → TeamCity)

Tested on cli.teamcity.com (tozd/go/errors). 9 GitLab jobs → 14 TC jobs (matrix expanded). 3 deploy jobs skipped (vendor-locked).

### Before (GitLab CI, abbreviated)

```yaml
variables:
  GIT_SUBMODULE_STRATEGY: recursive    # GitLab Runner internal — drop
  FF_ENABLE_BASH_EXIT_CODE_CHECK: "true"  # GitLab Runner internal — drop
  GOTOOLCHAIN: local

test:
  image: golang:$IMAGE_TAG              # needs docker-image: on TC steps
  before_script:                        # merge into script-content
    - apk --update add make bash gcc musl-dev
    - (cd /go; go install gotest.tools/gotestsum@$GOTESTSUM)
  script:
    - make test-ci
  parallel:
    matrix:                             # expand into separate TC jobs
      - IMAGE_TAG: ["1.21-alpine3.18", "1.22-alpine3.18", "1.23-alpine3.21"]
        GOTESTSUM: "v1.11.0"

lint:
  image: golang:1.26-alpine3.22
  script: [make lint-ci]

sync_releases:                          # skip — uses registry.gitlab.com image
  image: { name: registry.gitlab.com/tozd/gitlab/release/tag/v0-6-0:latest-debug }
  rules: [{ if: '$GITLAB_API_TOKEN && ...' }]
```

### After (TeamCity Pipeline YAML, abbreviated)

```yaml
jobs:
  test_go_1_21:
    name: "test go1.21"
    runs-on: Ubuntu-24.04-Large
    steps:
      - type: script
        docker-image: "golang:1.21-alpine3.18"
        script-content: |
          apk --update add make bash gcc musl-dev
          go install gotest.tools/gotestsum@v1.11.0
          GOTOOLCHAIN=local make test-ci
    files-publication:
      - path: "tests.xml"
        publish-artifact: true

  # test_go_1_22, test_go_1_23, ... (one job per matrix entry)

  lint:
    name: "lint"
    runs-on: Ubuntu-24.04-Large
    steps:
      - type: script
        docker-image: "golang:1.26-alpine3.22"
        script-content: |
          apk --update add make bash gcc musl-dev
          wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.10.1
          GOTOOLCHAIN=local make lint-ci

  # sync_releases, sync_config, publish — skipped (vendor-locked)
```

### Lessons from this migration (GitLab CI)

1. **`teamcity migrate` output needs heavy rework for GitLab CI.** The converter drops `image:` (no `docker-image:`), collapses `parallel: matrix` into one job with unresolved vars, and keeps GitLab Runner internals as parameters.
2. **`before_script` + `script` → single `script-content`.** The converter handles this correctly, but tool install commands from `before_script` must use hardcoded versions (not matrix variables) after expansion.
3. **Vendor-locked deploy jobs should be skipped, not stubbed.** Jobs using `registry.gitlab.com/*` images or `$CI_*` predefined variables have no TC equivalent.
4. **`GOTOOLCHAIN=local` goes inline, not as a parameter.** It's a Go build directive, not a CI variable — set it in the script where `go` runs.
