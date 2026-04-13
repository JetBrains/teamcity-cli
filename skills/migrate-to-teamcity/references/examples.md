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

