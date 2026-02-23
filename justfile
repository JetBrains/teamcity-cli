# TeamCity CLI

set quiet

# Build the CLI binary
build:
    go build -o bin/teamcity ./tc

# Format and lint the codebase
lint:
    go fmt ./...
    go fix ./...
    golangci-lint run --tests=false ./...

# Install the locally built CLI to $GOPATH/bin
install:
    go install ./tc

# Run unit tests
unit:
    TC_INSECURE_SKIP_WARN=1 go test -race -shuffle=on -v ./internal/config ./internal/errors ./internal/output ./internal/cmd

# Run all tests with coverage
test:
    TC_INSECURE_SKIP_WARN=1 go test -race -shuffle=on -v ./... -timeout 15m -tags=integration -coverprofile=coverage.out -coverpkg=./...

# Run acceptance tests against cli.teamcity.com (guest auth)
acceptance:
    TC_INSECURE_SKIP_WARN=1 go test -v -tags=acceptance ./acceptance -timeout 10m

# Remove build artifacts
[confirm]
clean:
    rm -rf bin/ dist/ .env coverage.out

# Generate CLI documentation
docs:
    go run scripts/generate-docs.go

# Run go generate
generate:
    go generate ./...

# Build a local snapshot release
snapshot:
    goreleaser release --snapshot --clean --skip=publish

# Test the release process without publishing
release-dry-run:
    goreleaser release --clean --skip=publish

# Create and publish a signed release
[confirm]
release:
    SIGN=true FINGERPRINT=B46DC71E03FEEB7F89D1F2491F7A8F87B9D8F501 goreleaser release --clean

# Record all documentation GIFs (both light and dark themes)
record-gifs *args:
    ./scripts/record-gifs.sh {{args}}

# Record only dark theme GIFs
record-gifs-dark *args:
    ./scripts/record-gifs.sh --dark-only {{args}}

# Record only light theme GIFs
record-gifs-light *args:
    ./scripts/record-gifs.sh --light-only {{args}}

# List available tape files for GIF recording
list-tapes:
    ./scripts/record-gifs.sh --list

# Build Writerside documentation using Docker (requires Rosetta enabled in Docker Desktop on Apple Silicon)
docs-build:
    #!/usr/bin/env bash
    set -euo pipefail
    rm -rf docs-out
    mkdir -p docs-out
    echo "Building Writerside docs..."
    docker run --rm --platform linux/amd64 \
        -v "$(pwd):/opt/sources" \
        -v "$(pwd)/docs-out:/opt/wrs-output" \
        -e SOURCE_DIR=/opt/sources \
        -e OUTPUT_DIR=/opt/wrs-output \
        -e MODULE_INSTANCE=docs/teamcity-cli \
        -e RUNNER=other \
        jetbrains/writerside-builder:latest
    echo "Build complete. Output in docs-out/"

# Deploy docs to gh-pages branch (run docs-build first, or build from Writerside IDE into docs-out/)
docs-deploy:
    #!/usr/bin/env bash
    set -euo pipefail
    # Find the webhelp zip from Docker build or IDE export
    ZIP=$(find docs-out -name "webHelp*.zip" -print -quit 2>/dev/null || true)
    if [[ -z "$ZIP" ]]; then
        echo "Error: No build output found in docs-out/."
        echo "Run 'just docs-build' or export from Writerside IDE into docs-out/."
        exit 1
    fi
    SITE="docs-out/site"
    rm -rf "$SITE"
    mkdir -p "$SITE"
    unzip -o "$ZIP" -d "$SITE"
    echo "Extracted $(find "$SITE" -type f | wc -l | tr -d ' ') files."
    # Deploy to gh-pages branch using a temporary worktree
    ROOT="$(pwd)"
    WORK=$(mktemp -d)
    trap 'cd "$ROOT"; git worktree remove "$WORK" --force 2>/dev/null; rm -rf "$WORK"' EXIT
    # Delete local gh-pages branch if it exists so we can create a fresh orphan
    git branch -D gh-pages 2>/dev/null || true
    git worktree add --detach "$WORK"
    cd "$WORK"
    git checkout --orphan gh-pages
    git rm -rf . > /dev/null 2>&1 || true
    cp -a "$ROOT/docs-out/site/." .
    touch .nojekyll
    git add -A
    git commit -m "Deploy Writerside docs to GitHub Pages"
    git push origin gh-pages --force
    echo "Deployed to gh-pages branch."

# Install JetBrains codesign client (requires JB employee VPN)
install-codesign:
    #!/usr/bin/env sh
    set -eu
    BASE_URL="https://codesign-distribution.labs.jb.gg"
    INSTALL_DIR="$HOME/.local/bin"
    OS="$(uname -s)"
    ARCH="$(uname -m)"
    case "$ARCH" in \
        x86_64) ARCH="amd64" ;; \
        aarch64|arm64) ARCH="arm64" ;; \
        *) echo "Unsupported architecture: $ARCH" && exit 1 ;; \
    esac
    case "$OS" in \
        Darwin) BINARY="codesign-client-darwin-$ARCH" ;; \
        Linux) BINARY="codesign-client-linux-$ARCH" ;; \
        MINGW*|MSYS*|CYGWIN*) BINARY="codesign-client-windows-amd64.exe" ;; \
        *) echo "Unsupported platform: $OS" && exit 1 ;; \
    esac
    mkdir -p "$INSTALL_DIR"
    echo "Downloading $BINARY to $INSTALL_DIR/codesign-client..."
    curl -fsSL "$BASE_URL/$BINARY" -o "$INSTALL_DIR/codesign-client"
    chmod +x "$INSTALL_DIR/codesign-client"
    echo "Installed codesign-client to $INSTALL_DIR/codesign-client"
    echo "Make sure $INSTALL_DIR is in your PATH"