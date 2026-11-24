"""Harbor verifier: grade what the agent produced and how it got there, and write the reward.

Runs inside the trial container (see harbor/Dockerfile) after Harbor copies the task's tests/ in.
Three things are graded and reported separately in checks.json:

  outcome  the answer file against tests/expected.json: the reward is the share of matching fields
  safety   gates read from the ATIF trajectory; any failure sets the reward to 0
  process  idiom checks from tests/checks.toml, advisory only

Oracle and no-op runs have no trajectory, so only their outcome is graded. An agent that left logs
but no trajectory was not measured: the verifier exits without a reward and Harbor records an error.
"""

from __future__ import annotations

import json
import sys
import tomllib
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from checks import CHECK_REGISTRY  # noqa: E402
from grade import grade, score  # noqa: E402
from scaffold.events import extract_events_atif  # noqa: E402
from scaffold.runner import EvalRunner  # noqa: E402

AGENT_LOGS = Path("/logs/agent")
OUT = Path("/logs/verifier")
WORKDIR = Path("/app")


def _run(events, check_ids: list[str]) -> dict:
    runner = EvalRunner(events)
    runner.run([CHECK_REGISTRY[c] for c in check_ids])
    return runner.summary()


def verify(tests: Path, workdir: Path, agent_logs: Path) -> dict:
    spec = tomllib.loads((tests / "checks.toml").read_text())
    outcome = grade(json.loads((tests / "expected.json").read_text()), workdir)
    result = {"outcome": {"score": score(outcome), "results": outcome}, "trajectory": False,
              "safety": {"results": []}, "results": [], "pass_rate": None}

    trajectory_file = agent_logs / "trajectory.json"
    trajectory = json.loads(trajectory_file.read_text()) if trajectory_file.is_file() else {}
    events = extract_events_atif(trajectory)
    if events.num_turns:
        process, safety = _run(events, spec.get("checks", [])), _run(events, spec.get("safety", []))
        result.update(trajectory=True, safety={"results": safety["results"]},
                      results=process["results"], pass_rate=process["pass_rate"])
    elif any(agent_logs.iterdir()) and not (agent_logs / "oracle.txt").is_file():
        raise SystemExit("the agent left logs but no trajectory: not a measurement")
    gated = any(not r["passed"] for r in result["safety"]["results"])
    result["reward"] = 0.0 if gated else result["outcome"]["score"]
    return result


def main(tests: str) -> None:
    result = verify(Path(tests), WORKDIR, AGENT_LOGS)
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "reward.json").write_text(json.dumps({"reward": result["reward"]}))
    (OUT / "checks.json").write_text(json.dumps(result, indent=2))
    for kind, results in (("outcome", result["outcome"]["results"]), ("safety", result["safety"]["results"]),
                          ("process", result["results"])):
        for r in results:
            print(f"{'PASS' if r['passed'] else 'FAIL'} {kind}/{r['check']}: {r['message']}")
    print(f"reward {result['reward']}")


if __name__ == "__main__":
    main(sys.argv[1])
