#!/usr/bin/env bash
# Air cloud startup script for teamcity-cli.
#
# Installs the Go toolchain + dev tools (just, gotestsum, golangci-lint),
# primes module/build caches, and builds the CLI. On a WARMUP run it also
# pulls the TeamCity server/agent Docker images used by the testcontainers
# integration tests and blocks on `healthcheck` so all of it lands in the
# environment snapshot. On a TASK run it exits promptly.
set -euo pipefail

log() { echo "[startup] $(date -u +%H:%M:%S) $*"; }

if [ "${AIR_STARTUP_MODE:-}" = warmup ]; then WARMUP=1; else WARMUP=; fi
log "mode: ${AIR_STARTUP_MODE:-task}"

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_DIR"

# --- PATH for this script and for the agent's future shells -----------------
GOPATH_DIR="$(go env GOPATH 2>/dev/null || echo "$HOME/go")"
GOBIN_DIR="$GOPATH_DIR/bin"
mkdir -p "$GOBIN_DIR" "$HOME/.local/bin"
export PATH="$GOBIN_DIR:$HOME/.local/bin:$PATH"

ENV_FILE="$HOME/.air-env.sh"
cat > "$ENV_FILE" <<EOF
export PATH="$GOBIN_DIR:\$HOME/.local/bin:\$PATH"
EOF
MARKER="# air-startup-env"
append_source() {
  grep -qF "$MARKER" "$1" 2>/dev/null || printf '\n%s\n. "$HOME/.air-env.sh"\n' "$MARKER" >> "$1"
}
# Login shells read only the first of these that exists.
for f in "$HOME/.bash_profile" "$HOME/.bash_login" "$HOME/.profile"; do
  if [ -f "$f" ]; then append_source "$f"; break; fi
done
[ -f "$HOME/.bash_profile" ] || [ -f "$HOME/.bash_login" ] || [ -f "$HOME/.profile" ] || append_source "$HOME/.profile"
append_source "$HOME/.bashrc"

# --- Go toolchain (preinstalled on the Air image; install if missing) -------
if ! go version 2>/dev/null | grep -q 'go1\.\(2[6-9]\|[3-9][0-9]\)'; then
  log "installing Go 1.26 from go.dev"
  ARCH=$(dpkg --print-architecture)
  curl -fsSL 'https://go.dev/dl/?mode=json' -o /tmp/go-dl.json
  GO_VERSION=$(python3 -c 'import json; print(next(r["version"] for r in json.load(open("/tmp/go-dl.json")) if r["version"].startswith("go1.26.")))')
  curl -fsSL "https://go.dev/dl/${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf /tmp/go.tar.gz
  rm -f /tmp/go.tar.gz /tmp/go-dl.json
  echo 'export PATH="/usr/local/go/bin:$PATH"' >> "$ENV_FILE"
  export PATH="/usr/local/go/bin:$PATH"
fi
log "$(go version)"

# --- Dev tools ---------------------------------------------------------------
# just >= 1.23 is required (justfile uses `set quiet`); apt ships 1.21, so
# install a pinned release binary from GitHub.
JUST_VERSION=1.59.0
if ! just --version 2>/dev/null | grep -qE 'just 1\.(2[3-9]|[3-9][0-9])'; then
  log "installing just ${JUST_VERSION}"
  case "$(uname -m)" in
    aarch64|arm64) JUST_ARCH=aarch64 ;;
    *) JUST_ARCH=x86_64 ;;
  esac
  curl -fsSL "https://github.com/casey/just/releases/download/${JUST_VERSION}/just-${JUST_VERSION}-${JUST_ARCH}-unknown-linux-musl.tar.gz" -o /tmp/just.tar.gz
  tar -C "$HOME/.local/bin" -xzf /tmp/just.tar.gz just
  rm -f /tmp/just.tar.gz
fi
log "$(just --version)"

if ! command -v gotestsum >/dev/null 2>&1; then
  log "installing gotestsum"
  go install gotest.tools/gotestsum@latest
fi

if ! command -v golangci-lint >/dev/null 2>&1; then
  # Built from source via the Go module proxy: the binary-release installer's
  # checksum verification fails behind the environment's egress proxy.
  log "installing golangci-lint (go install)"
  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
fi
log "$(golangci-lint version 2>&1 | head -1)"

# --- Project dependencies and build ------------------------------------------
log "go mod download"
go mod download

log "building CLI (just build)"
just build
log "built: $(./bin/teamcity --version 2>&1 | head -1)"

# --- Warmup-only: prime caches and testcontainers images ---------------------
if [ -n "${WARMUP:-}" ]; then
  # Integration tests (just test) spin up these images via testcontainers;
  # pulling them now bakes the layers into the snapshot. Run detached,
  # healthcheck waits for completion.
  rm -f /tmp/tc-image-pull.done
  nohup bash -c '
    ok=1
    for img in jetbrains/teamcity-server:latest jetbrains/teamcity-agent:latest; do
      echo "pulling $img"
      docker pull "$img" || ok=0
    done
    if [ "$ok" = 1 ]; then echo ok > /tmp/tc-image-pull.done; else echo failed > /tmp/tc-image-pull.done; fi
  ' > /tmp/tc-image-pull.log 2>&1 &

  log "priming build cache (go build ./...)"
  go build ./...
fi

healthcheck() {
  log "healthcheck: unit tests (just unit)"
  just unit

  log "healthcheck: waiting for Docker daemon"
  until docker info >/dev/null 2>&1; do
    log "healthcheck: docker not ready yet"
    sleep 5
  done

  log "healthcheck: waiting for TeamCity image pulls"
  while [ ! -f /tmp/tc-image-pull.done ]; do
    log "healthcheck: still pulling images ($(tail -1 /tmp/tc-image-pull.log 2>/dev/null | cut -c1-80))"
    sleep 15
  done
  if [ "$(cat /tmp/tc-image-pull.done)" != ok ]; then
    log "healthcheck: image pull FAILED"
    tail -30 /tmp/tc-image-pull.log
    return 1
  fi
  docker image inspect jetbrains/teamcity-server:latest jetbrains/teamcity-agent:latest --format '{{.RepoTags}}' >/dev/null

  log "healthcheck: OK"
}

if [ -n "${WARMUP:-}" ]; then
  healthcheck
fi
log "startup complete"
