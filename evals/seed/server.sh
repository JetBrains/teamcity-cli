#!/usr/bin/env bash
# Start or stop the seeded TeamCity server that eval trials query.
#
# Usage: eval "$(evals/seed/server.sh up)"   exports TEAMCITY_URL and TEAMCITY_TOKEN (read-only viewer)
#        evals/seed/server.sh down
#        evals/seed/server.sh push            publishes the local snapshot to TC_EVAL_SNAPSHOT_REPO
#
# The server runs from the snapshot image built by `just eval-seed` and joins the `tc-eval` Docker
# network as `teamcity`; trial containers join the same network (harbor/compose.network.yaml).
# With TC_EVAL_SNAPSHOT_REPO set (a private registry repository), a missing or stale local snapshot
# is pulled from there, tagged with the fingerprint in manifest.json and the architecture.
set -euo pipefail
cd "$(dirname "$0")"

image="teamcity-cli-eval-server:seeded"
name="tc-eval-server"
network="tc-eval"
host_url="http://127.0.0.1:${TC_EVAL_PORT:-18111}"

want="$(python3 -c 'import json; print(json.load(open("manifest.json"))["fingerprint"])')"
# Snapshots are per architecture: a run pulls the host's, a push publishes whatever the local snapshot is.
arch="$(docker image inspect -f '{{.Architecture}}' "$image" 2> /dev/null || true)"
[ "${1:-}" = push ] || arch="$(docker version -f '{{.Server.Arch}}')"
remote="${TC_EVAL_SNAPSHOT_REPO:-}:$want-$arch"

fingerprint() { docker image inspect -f '{{index .Config.Labels "tc.eval.fingerprint"}}' "$image" 2> /dev/null || true; }

up() {
  if [ "$(fingerprint)" != "$want" ] && [ -n "${TC_EVAL_SNAPSHOT_REPO:-}" ]; then
    docker pull -q "$remote" > /dev/null && docker tag "$remote" "$image"
  fi
  have="$(fingerprint)"
  [ -n "$have" ] || { echo "no $image image: run 'just eval-seed' or set TC_EVAL_SNAPSHOT_REPO" >&2; exit 1; }
  [ "$want" = "$have" ] || { echo "snapshot $have does not match manifest.json $want: run 'just eval-seed'" >&2; exit 1; }
  docker network inspect "$network" > /dev/null 2>&1 || docker network create "$network" > /dev/null
  if [ -z "$(docker ps -q -f "name=^$name\$")" ]; then
    docker rm -f "$name" > /dev/null 2>&1 || true
    docker run -d --name "$name" --network "$network" --network-alias teamcity \
      -p "127.0.0.1:${TC_EVAL_PORT:-18111}:8111" "$image" > /dev/null
  fi
  token="$(docker exec "$name" cat /opt/teamcity/eval-token)"
  curl -sf --retry 60 --retry-delay 3 --retry-all-errors -o /dev/null \
    -H "Authorization: Bearer $token" "$host_url/app/rest/server" || { echo "$name did not start" >&2; exit 1; }
  echo "export TEAMCITY_URL=http://teamcity:8111 TEAMCITY_TOKEN=$token"
}

case "${1:-}" in
  up) up ;;
  down) docker rm -f -v "$name" > /dev/null 2>&1 || true ;;
  push)
    : "${TC_EVAL_SNAPSHOT_REPO:?}"
    [ "$(fingerprint)" = "$want" ] || { echo "local snapshot does not match manifest.json $want" >&2; exit 1; }
    docker tag "$image" "$remote" && docker push "$remote" ;;
  *) echo "usage: $0 up|down|push" >&2; exit 2 ;;
esac
