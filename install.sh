#!/bin/bash

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

RELEASE=${1}
TMP_DIR="/tmp/tmpinstalldir"
OUT_DIR=${2:-"/usr/local/bin"}
check_mark="\033[1;32m✓\033[0m"

echo -e '
 ████████╗ ██████╗
 ╚══██╔══╝██╔════╝   TeamCity CLI (installer)
    ██║   ██║        Documentation
    ██║   ██║        https://jb.gg/tc/docs
    ██║   ╚██████╗   Report issues
    ╚═╝    ╚═════╝   https://jb.gg/tc/issues
    ══════════════
'

echo -e "
This script will download TeamCity CLI to \033[4m$OUT_DIR/teamcity\033[0m

If you get 'permission denied' error:
  - Specify other dir: \033[4mcurl -fsSL https://jb.gg/tc/install | bash -s -- \"\" \$HOME/.local/bin\033[0m
  - Or run with sudo

If you get API rate limit exceeded error:
  - Specify the version: \033[4mcurl -fsSL https://jb.gg/tc/install | bash -s -- v1.0.0\033[0m
  - Or set GITHUB_TOKEN: \033[4mexport GITHUB_TOKEN=your_token\033[0m
"
set -e

function cleanup {
    rm -rf $TMP_DIR > /dev/null 2>&1 || true
}
function header() {
    echo -e "\n\033[1m$1\033[0m";
}
function fail {
    cleanup
    msg=$1
    echo "============"
    echo "Error: $msg" 1>&2
    exit 1
}
function install {
    set -e
    if [ -z "$RELEASE" ]; then
        GITHUB_AUTH=""
        if [ -n "$GITHUB_TOKEN" ]; then
            GITHUB_AUTH="-H Authorization: token $GITHUB_TOKEN"
        fi
        LATEST=$(curl $GITHUB_AUTH --silent "https://api.github.com/repos/JetBrains/teamcity-cli/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        RELEASE=$LATEST
    fi
    USER="JetBrains"
    PROG="teamcity"
    INSECURE="false"
    #bash check
    [ ! "$BASH_VERSION" ] && fail "Please use bash instead"
    [ ! -d $OUT_DIR ] && fail "output directory missing: $OUT_DIR"
    #dependency check, assume we are a standard POSIX machine
    which find > /dev/null || fail "find not installed"
    which xargs > /dev/null || fail "xargs not installed"
    which sort > /dev/null || fail "sort not installed"
    which tail > /dev/null || fail "tail not installed"
    which cut > /dev/null || fail "cut not installed"
    which du > /dev/null || fail "du not installed"
    GET=""
    if which curl > /dev/null; then
        GET="curl"
        if [[ $INSECURE = "true" ]]; then GET="$GET --insecure"; fi
        GET="$GET --fail -# -L"
    elif which wget > /dev/null; then
        GET="wget"
        if [[ $INSECURE = "true" ]]; then GET="$GET --no-check-certificate"; fi
        GET="$GET -qO-"
    else
        fail "neither wget/curl are installed"
    fi
    case $(uname -s) in
    Darwin) OS="darwin";;
    Linux) OS="linux";;
    *) fail "unknown os: $(uname -s)";;
    esac
    #find ARCH
    if [[ $(uname -m) == "x86_64" ]]; then
        ARCH="x86_64"
    elif [[ $(uname -m) == "aarch64" || $(uname -m) == "arm64" ]]; then
        ARCH="arm64"
    else
        fail "unknown arch: $(uname -m)"
    fi
    URL="https://github.com/JetBrains/teamcity-cli/releases/download/$RELEASE/teamcity_${RELEASE#v}_${OS}_${ARCH}.tar.gz"
    FTYPE=".tar.gz"

    echo -e "\033[0;90m\nInstalling $PROG ($RELEASE) from $URL\033[0m\n"

    #enter tempdir
    mkdir -p $TMP_DIR
    cd $TMP_DIR || exit
    if [[ $FTYPE = ".tar.gz" ]] || [[ $FTYPE = ".tgz" ]]; then
        which tar > /dev/null || fail "tar is not installed"
        which gzip > /dev/null || fail "gzip is not installed"
        bash -c "$GET $URL" | tar zxf - > /dev/null || fail "download failed"
    else
        fail "unknown file type: $FTYPE"
    fi
    TMP_BIN=$(find . -name "teamcity" -type f | head -1)
    if [ ! -f "$TMP_BIN" ]; then
        fail "could not find teamcity binary"
    fi
    chmod +x "$TMP_BIN" || fail "chmod +x failed"
    mv "$TMP_BIN" "$OUT_DIR"/$PROG || fail "mv failed"
    echo -e "${check_mark} Installed at $OUT_DIR/$PROG\n"
    "$OUT_DIR/$PROG" --version
    cleanup
    header "Next steps"
    echo -e ""
    echo -e "  \033[1mAuthenticate with TeamCity\033[0m"
    echo -e "  \033[0;90mteamcity auth login\033[0m\n"
    echo -e "  \033[1mList recent builds\033[0m"
    echo -e "  \033[0;90mteamcity run list\033[0m\n"
    echo -e "  \033[1mGet help\033[0m"
    echo -e "  \033[0;90mteamcity --help\033[0m\n"
}

install
