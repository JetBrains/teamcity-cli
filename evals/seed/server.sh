#!/usr/bin/env bash
# Start or stop the seeded TeamCity server that eval trials query.
#
# Usage: eval "$(evals/seed/server.sh up)"   exports TEAMCITY_URL and TEAMCITY_TOKEN (read-only viewer)
#        evals/seed/server.sh down
#
# The server runs from the snapshot image built by `just eval-seed` and joins the `tc-eval` Docker
# network as `teamcity`; trial containers join the same network (harbor/compose.network.yaml).
set -euo pipefail
cd "$(dirname "$0")"

image="teamcity-cli-eval-server:seeded"
name="tc-eval-server"
network="tc-eval"
host_url="http://127.0.0.1:${TC_EVAL_PORT:-18111}"

up() {
  docker image inspect "$image" > /dev/null 2>&1 || { echo "no $image image: run 'just eval-seed' first" >&2; exit 1; }
  want="$(python3 -c 'import json; print(json.load(open("manifest.json"))["fingerprint"])')"
  have="$(docker image inspect -f '{{index .Config.Labels "tc.eval.fingerprint"}}' "$image")"
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
  *) echo "usage: $0 up|down" >&2; exit 2 ;;
esac
