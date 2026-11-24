"""Convert Harbor jobs into result artifacts that compare.py and report.py read.

Usage: harbor_import.py <jobs-dir> <results-dir>

Each job directory under <jobs-dir> is one treatment and is named after it
(CONTROL, CURRENT). The task comes from each trial's config.json, per-check results from its
verifier output (checks.json), and efficiency metrics from its ATIF trajectory.

Like the pytest runner, it also grades llm_grade tasks with the LLM judge when model
credentials are set, and sends every trial to Langfuse when LANGFUSE_* keys are set.
Both run here, on the host, so no judge or Langfuse secrets enter the trial containers.
"""

from __future__ import annotations

import json
import os
import sys
from datetime import datetime
from pathlib import Path

EVALS_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(EVALS_ROOT))

from scaffold import langfuse_log  # noqa: E402
from scaffold.events import extract_events_atif  # noqa: E402
from scaffold.graders import grade_all  # noqa: E402
from scaffold.tasks import load_task  # noqa: E402

_JUDGE_CREDS = ("ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN")


def _seconds(span: dict | None) -> float:
    if not span or not span.get("started_at") or not span.get("finished_at"):
        return 0.0
    def parse(s: str) -> datetime:
        return datetime.fromisoformat(s.replace("Z", "+00:00"))
    return (parse(span["finished_at"]) - parse(span["started_at"])).total_seconds()


def _grades(task, text: str) -> tuple[list[dict], list[str]]:
    if not (task.llm_grade and text and any(os.environ.get(k) for k in _JUDGE_CREDS)):
        return [], []
    grades = grade_all(task.instruction, text)
    graded = [{"dimension": d, "score": g.score, "reasoning": g.reasoning} for d, g in grades.items() if g]
    return graded, [d for d, g in grades.items() if g is None]


def import_jobs(jobs_dir: Path, results_dir: Path) -> int:
    results_dir.mkdir(parents=True, exist_ok=True)
    count = errored = 0
    for job in sorted(p for p in jobs_dir.iterdir() if p.is_dir()):
        for trial in sorted(p for p in job.iterdir() if (p / "config.json").is_file()):
            config = json.loads((trial / "config.json").read_text())
            task = load_task(Path(config["task"]["path"]).name)
            result_file = trial / "result.json"
            result = json.loads(result_file.read_text()) if result_file.is_file() else {}
            verifier_out = trial / "verifier" / "checks.json"
            if verifier_out.is_file() and result.get("reward_valid", True):
                checks = json.loads(verifier_out.read_text())
            else:  # the agent or environment failed (Harbor marks the reward invalid): score it failed
                reason = (result.get("exception_info") or {}).get("exception_type") or "no verifier output"
                print(f"warning: {trial.name}: {reason}; scoring it as failed", file=sys.stderr)
                errored += 1
                checks = {"pass_rate": 0.0, "results": [
                    {"check": c, "passed": False, "message": f"trial error: {reason}"} for c in task.checks]}
            trajectory_file = trial / "agent" / "trajectory.json"
            events = extract_events_atif(json.loads(trajectory_file.read_text()) if trajectory_file.is_file() else {})
            agent = config.get("agent") or {}
            graded, ungraded = _grades(task, events.full_text)
            checks.update(task=task.name, task_id=task.name, treatment=job.name, events=events.summary(),
                          llm_grades=graded, ungraded_dimensions=ungraded)
            (results_dir / f"{task.name}_{job.name}_{trial.name}.json").write_text(json.dumps(checks, indent=2))

            langfuse_log.log_run(
                task_name=task.name, treatment_name=job.name, instruction=task.instruction,
                experiment_id=results_dir.name, response_text=events.full_text,
                pass_rate=checks.get("pass_rate", 0.0), duration_sec=_seconds(result.get("agent_execution")),
                num_turns=events.num_turns, input_tokens=events.input_tokens, output_tokens=events.output_tokens,
                skill_invoked=any("teamcity" in s.lower() for s in events.skills_invoked),
                skill_available=bool(agent.get("skills")), check_results=checks.get("results", []),
                tool_calls=events.tool_calls, tool_results=events.tool_results, llm_grades=graded,
                ungraded_dimensions=ungraded, run_id=trial.name,
                provenance={"model": agent.get("model_name"), "agent": agent.get("name"), "runner": "harbor"},
            )
            count += 1
    langfuse_log.flush()
    if count and errored * 2 > count:
        raise SystemExit(f"{errored}/{count} trials errored: an infrastructure failure, not a measurement")
    return count


if __name__ == "__main__":
    n = import_jobs(Path(sys.argv[1]), Path(sys.argv[2]))
    print(f"Imported {n} trial(s) into {sys.argv[2]}")
