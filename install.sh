#!/usr/bin/env bash

#
# Copyright 2021-2026 JetBrains s.r.o.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -Eeuo pipefail

PROG="teamcity"
CHECK_MARK=$'\033[1;32m✓\033[0m'
TMP_DIR=""
STAGED_BIN=""
JETBRAINS_CDN=0

_POSITIONAL=()
for _arg in "$@"; do
    case "$_arg" in
        --jetbrains-cdn) JETBRAINS_CDN=1 ;;
        *) _POSITIONAL+=("$_arg") ;;
    esac
done
RELEASE="${_POSITIONAL[0]:-}"
OUT_DIR="${_POSITIONAL[1]:-/usr/local/bin}"
unset _arg _POSITIONAL

cleanup() {
    if [[ -n "${TMP_DIR:-}" && -d "$TMP_DIR" ]]; then
        rm -rf "$TMP_DIR"
    fi
    if [[ -n "${STAGED_BIN:-}" && -e "$STAGED_BIN" ]]; then
        rm -f "$STAGED_BIN"
    fi
}

header() {
    printf '\n\033[1m%s\033[0m\n' "$1"
}

fail() {
    local msg="${1:-unknown error}"
    printf '============\n' >&2
    printf 'Error: %s\n' "$msg" >&2
    exit 1
}

need_cmd() {
    command -v "$1" >/dev/null 2>&1 || fail "$1 is not installed"
}

detect_os() {
    case "$(uname -s)" in
        Darwin) printf 'darwin' ;;
        Linux) printf 'linux' ;;
        *) fail "unknown os: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) printf 'x86_64' ;;
        aarch64|arm64) printf 'arm64' ;;
        *) fail "unknown arch: $(uname -m)" ;;
    esac
}

download_to_stdout() {
    local url="$1"

    if command -v curl >/dev/null 2>&1; then
        curl --fail --location --silent --show-error "$url"
        return
    fi

    if command -v wget >/dev/null 2>&1; then
        wget -qO- "$url"
        return
    fi

    fail "neither curl nor wget is installed"
}

resolve_latest_release() {
    if [[ "$JETBRAINS_CDN" == 1 ]]; then
        local tag
        tag="$(download_to_stdout "https://download.jetbrains.com/resources/teamcity-cli/latest" 2>/dev/null | tr -d '[:space:]')"
        [[ -n "$tag" ]] || fail "failed to resolve latest TeamCity CLI release from JetBrains CDN"
        printf '%s' "$tag"
        return
    fi

    local location tag

    if command -v curl >/dev/null 2>&1; then
        location="$(curl -sI "https://github.com/JetBrains/teamcity-cli/releases/latest" 2>/dev/null \
            | grep -i '^location:' | head -n 1 | tr -d '\r')"
    elif command -v wget >/dev/null 2>&1; then
        location="$(wget --spider -S "https://github.com/JetBrains/teamcity-cli/releases/latest" 2>&1 \
            | grep -i '^\s*location:' | tail -n 1 | tr -d '\r' | sed 's/^\s*//')"
    else
        fail "neither curl nor wget is installed"
    fi

    tag="${location##*/}"
    [[ -n "$tag" && "$tag" != "$location" ]] || fail "failed to resolve latest TeamCity CLI release"
    printf '%s' "$tag"
}

install_teamcity() {
    [[ -n "${BASH_VERSION:-}" ]] || fail "please use bash"

    need_cmd tar
    need_cmd find
    need_cmd chmod
    need_cmd mv
    need_cmd mkdir
    need_cmd mktemp
    need_cmd uname
    need_cmd cp

    if [[ -z "$RELEASE" ]]; then
        RELEASE="$(resolve_latest_release)"
    fi

    [[ "$RELEASE" =~ ^v?[0-9A-Za-z._-]+$ ]] || fail "invalid release: $RELEASE"

    local os arch version gh_url jb_url tmp_bin target
    os="$(detect_os)"
    arch="$(detect_arch)"
    version="${RELEASE#v}"
    gh_url="https://github.com/JetBrains/teamcity-cli/releases/download/${RELEASE}/${PROG}_${version}_${os}_${arch}.tar.gz"
    jb_url="https://download.jetbrains.com/resources/teamcity-cli/${version}/${PROG}_${version}_${os}_${arch}.tar.gz"
    target="${OUT_DIR}/${PROG}"

    echo -e "\033[0;90m\nInstalling $PROG ($RELEASE)\033[0m\n"

    mkdir -p "$OUT_DIR" || fail "failed to create output directory: $OUT_DIR"
    [[ -d "$OUT_DIR" ]] || fail "output path is not a directory: $OUT_DIR"
    [[ -w "$OUT_DIR" ]] || fail "output directory is not writable: $OUT_DIR"

    TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PROG}.XXXXXX")" || fail "failed to create temp directory"
    cd "$TMP_DIR"

    if [[ "$JETBRAINS_CDN" == 1 ]]; then
        download_to_stdout "$jb_url" | tar -xzf - || fail "download or extract failed"
    else
        download_to_stdout "$gh_url" | tar -xzf - || fail "download or extract failed

If GitHub is unreachable, retry using the JetBrains CDN:
  curl -fsSL https://jb.gg/tc/install | bash -s -- ${RELEASE:+$RELEASE }--jetbrains-cdn"
    fi

    tmp_bin="$(find . -type f -name "$PROG" -print -quit)"
    [[ -n "$tmp_bin" && -f "$tmp_bin" ]] || fail "could not find ${PROG} binary in archive"

    STAGED_BIN="$(mktemp "${OUT_DIR}/.${PROG}.XXXXXX")" || fail "failed to create staged binary in $OUT_DIR"
    cp "$tmp_bin" "$STAGED_BIN" || fail "failed to stage binary"
    chmod 0755 "$STAGED_BIN" || fail "failed to set executable permissions"
    mv -f "$STAGED_BIN" "$target" || fail "failed to move binary into place"
    STAGED_BIN=""

    echo -e "${CHECK_MARK} Installed at $target\n"
    "$target" --version || fail "installed binary failed to run"

    header "Next steps"
    echo -e ""
    echo -e "  \033[1mAuthenticate with TeamCity\033[0m"
    echo -e "  \033[0;90mteamcity auth login\033[0m\n"
    echo -e "  \033[1mList recent builds\033[0m"
    echo -e "  \033[0;90mteamcity run list\033[0m\n"
    echo -e "  \033[1mGet help\033[0m"
    echo -e "  \033[0;90mteamcity --help\033[0m\n"
}

trap cleanup EXIT
trap 'fail "interrupted"' INT TERM

echo -e '
 ████████╗ ██████╗
 ╚══██╔══╝██╔════╝   TeamCity CLI (installer)
    ██║   ██║        Documentation
    ██║   ██║        https://jb.gg/tc/docs
    ██║   ╚██████╗   Report issues
    ╚═╝    ╚═════╝   https://jb.gg/tc/issues
'

echo -e "
This script will download TeamCity CLI to \033[4m$OUT_DIR/teamcity\033[0m

If you get 'permission denied' error:
  - Specify other dir: \033[4mcurl -fsSL https://jb.gg/tc/install | bash -s -- \"\" \$HOME/.local/bin\033[0m
  - Or run with sudo

To install a specific version:
  \033[4mcurl -fsSL https://jb.gg/tc/install | bash -s -- v0.8.3\033[0m

If GitHub is down, use JetBrains CDN directly:
  \033[4mcurl -fsSL https://jb.gg/tc/install | bash -s -- --jetbrains-cdn\033[0m
"

install_teamcity
