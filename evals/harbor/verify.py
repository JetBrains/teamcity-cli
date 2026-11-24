"""Harbor verifier: grade the agent's ATIF trajectory with the task's checks and write the reward.

Runs inside the trial container (see harbor/Dockerfile). The reward is the fraction of checks that
passed, matching the pass rate the pytest harness reports; per-check results go to reward.json.
"""

from __future__ import annotations

import json
import sys
import tomllib
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from checks import CHECK_REGISTRY  # noqa: E402
from scaffold.events import extract_events_atif  # noqa: E402
from scaffold.runner import EvalRunner  # noqa: E402

TRAJECTORY = Path("/logs/agent/trajectory.json")
OUT = Path("/logs/verifier")


def main(checks_file: str) -> None:
    check_ids = tomllib.loads(Path(checks_file).read_text())["checks"]
    trajectory = json.loads(TRAJECTORY.read_text()) if TRAJECTORY.is_file() else {}
    runner = EvalRunner(extract_events_atif(trajectory), task_name=Path(checks_file).parent.parent.name)
    runner.run([CHECK_REGISTRY[c] for c in check_ids])
    summary = runner.summary()
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "reward.json").write_text(json.dumps({"reward": summary["pass_rate"]}))
    (OUT / "checks.json").write_text(json.dumps(summary, indent=2))
    for r in summary["results"]:
        print(("PASS " if r["passed"] else "FAIL ") + r["check"] + ": " + r["message"])


if __name__ == "__main__":
    main(sys.argv[1])
