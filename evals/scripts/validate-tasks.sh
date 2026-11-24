#!/usr/bin/env bash
# Check the tasks without any model: on every task the oracle (solution/solve.sh) must score 1
# and the no-op agent must score 0. Run it after changing a task, a grader or the seed.
#
# Usage: evals/scripts/validate-tasks.sh [harbor run args]   e.g. -i flaky-tests
set -euo pipefail
cd "$(dirname "$0")/../.."

[ -n "$(docker ps -q -f 'name=^tc-eval-server$')" ] || trap "'$PWD/evals/seed/server.sh' down" EXIT
eval "$("$PWD/evals/seed/server.sh" up)"
GOOS=linux go build -o bin/teamcity-linux ./tc
go run scripts/generate-cli-schema.go > evals/cli_schema.json
docker build -q -f evals/harbor/Dockerfile --build-arg TC_EVAL_URL="$TEAMCITY_URL" --build-arg TC_EVAL_TOKEN="$TEAMCITY_TOKEN" \
  -t teamcity-cli-eval:latest . > /dev/null

cd evals
out="results/validate"
rm -rf "$out"
export HARBOR_TELEMETRY=off
for agent in oracle nop; do
  harbor run -y -e docker -p tasks -a "$agent" -o "$out" --job-name "$agent" \
    --extra-docker-compose "$PWD/harbor/compose.network.yaml" "$@"
done

python3 - "$out" <<'EOF'
import json, sys
from pathlib import Path

want, bad, seen = {"oracle": 1.0, "nop": 0.0}, [], 0
for agent, expected in want.items():
    for trial in sorted(p for p in (Path(sys.argv[1]) / agent).iterdir() if (p / "config.json").is_file()):
        reward_file = trial / "verifier" / "reward.json"
        reward = json.loads(reward_file.read_text())["reward"] if reward_file.is_file() else None
        seen += 1
        if reward != expected:
            bad.append(f"{agent} {trial.name}: reward {reward}, expected {expected}")
print(f"{seen - len(bad)}/{seen} trials as expected (oracle 1, no-op 0)")
print("\n".join(bad))
sys.exit(1 if bad or not seen else 0)
EOF
