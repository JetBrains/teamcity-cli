"""Convert Harbor jobs into result artifacts that compare.py and report.py read.

Usage: harbor_import.py <jobs-dir> <results-dir>

Each job directory under <jobs-dir> is one treatment and is named after it
(CONTROL, CURRENT). The task comes from each trial's config.json, per-check results from its
verifier output (checks.json), and efficiency metrics from its ATIF trajectory.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

EVALS_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(EVALS_ROOT))

from scaffold.events import extract_events_atif  # noqa: E402


def import_jobs(jobs_dir: Path, results_dir: Path) -> int:
    results_dir.mkdir(parents=True, exist_ok=True)
    count = 0
    for job in sorted(p for p in jobs_dir.iterdir() if p.is_dir()):
        for trial in sorted(p for p in job.iterdir() if (p / "verifier" / "checks.json").is_file()):
            checks = json.loads((trial / "verifier" / "checks.json").read_text())
            trajectory_file = trial / "agent" / "trajectory.json"
            trajectory = json.loads(trajectory_file.read_text()) if trajectory_file.is_file() else {}
            task = Path(json.loads((trial / "config.json").read_text())["task"]["path"]).name
            checks.update(task=task, task_id=task, treatment=job.name,
                          events=extract_events_atif(trajectory).summary())
            (results_dir / f"{task}_{job.name}_{trial.name}.json").write_text(json.dumps(checks, indent=2))
            count += 1
    return count


if __name__ == "__main__":
    n = import_jobs(Path(sys.argv[1]), Path(sys.argv[2]))
    print(f"Imported {n} trial(s) into {sys.argv[2]}")
