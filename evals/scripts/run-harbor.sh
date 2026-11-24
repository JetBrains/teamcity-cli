#!/usr/bin/env bash
# Run evals/tasks/ in Harbor on local Docker: a CONTROL job without the skill and a CURRENT job
# with it, then import the trials (checks, LLM judge, Langfuse), gate them and write report.html.
#
# Usage: evals/scripts/run-harbor.sh [harbor run args]   e.g. -k 2 -n 4, or -i flaky-tests
# Env:   TEAMCITY_URL, TEAMCITY_TOKEN (eval server), ANTHROPIC_API_KEY (agent and judge),
#        EVAL_MODEL (default anthropic/claude-sonnet-5-5), BRANCH_NAME (names the experiment),
#        LANGFUSE_* (optional), GATE_MODE (warn|enforce).
set -euo pipefail
cd "$(dirname "$0")/../.."

: "${TEAMCITY_URL:?}" "${TEAMCITY_TOKEN:?}"
model="${EVAL_MODEL:-anthropic/claude-sonnet-5-5}"
experiment="$(printf '%s' "${BRANCH_NAME:-harbor-$(date +%Y%m%d-%H%M%S)}" | tr '/:()' '_-__')"

GOOS=linux go build -o bin/teamcity-linux ./tc
go run scripts/generate-cli-schema.go > evals/cli_schema.json
docker build -q -f evals/harbor/Dockerfile -t teamcity-cli-eval:latest . > /dev/null

cd evals
uv sync -q
uv run pytest tests -q  # harness unit tests: fail fast before any agent runs
exp="results/$experiment"
rm -rf "$exp"
# The module cache is read-only; Harbor needs to copy the skill's session files back out.
skill="$(mktemp -d)/teamcity-cli"
cp -R "$(uv run python -m scaffold.skill)" "$skill" && chmod -R u+w "$skill"

export HARBOR_TELEMETRY=off
# A local API proxy on loopback is unreachable from the trial containers; point them at the host.
run_harbor() {
  if [ -n "${ANTHROPIC_BASE_URL:-}" ]; then
    ANTHROPIC_BASE_URL="$(printf '%s' "$ANTHROPIC_BASE_URL" | sed -E 's#//(127\.0\.0\.1|localhost)([:/])#//host.docker.internal\2#')" harbor "$@"
  else
    harbor "$@"
  fi
}
run_harbor run -y -e docker -p tasks -a claude-code -m "$model" -o "$exp/jobs" --job-name CONTROL -x negative-unrelated "$@"
run_harbor run -y -e docker -p tasks -a claude-code -m "$model" -o "$exp/jobs" --job-name CURRENT --skill "$skill" "$@"

uv run python scripts/harbor_import.py "$exp/jobs" "$exp"
status=0
uv run python scripts/compare.py "$exp" || status=$?
uv run python scripts/report.py "$exp"
exit "$status"
