# TeamCity CLI

set quiet

# Build the CLI binary
build:
    go build -o bin/tc ./tc

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
    TC_INSECURE_SKIP_WARN=1 go test -v ./internal/config ./internal/errors ./internal/output ./internal/cmd

# Run all tests with coverage
test:
    TC_INSECURE_SKIP_WARN=1 go test -v ./... -timeout 15m -tags=integration -coverprofile=coverage.out -coverpkg=./...

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