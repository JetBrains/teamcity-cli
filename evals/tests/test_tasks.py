"""Main eval test runner.

Usage:
    just eval                                    # all tasks, default treatments
    just eval-task investigate-failure            # single task
    just eval-compare investigate-failure         # CONTROL vs CURRENT
    just eval --runs=3 -n 4                      # parallel with repetitions
"""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path

import pytest

from scaffold.claude import run_claude, run_claude_docker
from scaffold.events import extract_events
from scaffold.graders import grade_all
from scaffold.runner import EvalRunner
from scaffold import sentry_log
from scaffold.tasks import TaskConfig
from conftest import TreatmentConfig
from checks import CHECK_REGISTRY

EVALS_ROOT = Path(__file__).resolve().parent.parent
if str(EVALS_ROOT) not in sys.path:
    sys.path.insert(0, str(EVALS_ROOT))


@pytest.mark.timeout(900)
def test_task(
    task_config: TaskConfig,
    treatment_config: TreatmentConfig,
    experiment_id: str,
    run_id: str,
) -> None:
    # --- Execute Claude ---
    use_docker = not os.environ.get("BENCH_LOCAL")
    timeout = int(os.environ.get("BENCH_TIMEOUT", "600"))
    model = os.environ.get("BENCH_CC_MODEL")

    execute = run_claude_docker if use_docker else run_claude
    result = execute(
        prompt=task_config.instruction,
        treatment=treatment_config,
        model=model,
        timeout=timeout,
    )

    # --- Parse events ---
    events = extract_events(result.raw_output)

    # --- Run checks from CHECK_REGISTRY ---
    runner = EvalRunner(events, task_name=f"{task_config.name}/{treatment_config.name}")
    check_fns = [CHECK_REGISTRY[c] for c in task_config.checks]
    runner.run(check_fns)

    # --- LLM grading (only tasks that declare it in tasks.json) ---
    llm_grades = []
    if task_config.llm_grade and os.environ.get("ANTHROPIC_API_KEY") and runner.text:
        llm_grades = grade_all(task_config.instruction, runner.text)
        for g in llm_grades:
            if g.passed:
                runner.passed(f"[LLM] {g.dimension}: {g.score}/5 — {g.reasoning}")
            else:
                runner.failed(f"[LLM] {g.dimension}: {g.score}/5 — {g.reasoning}")

    # --- Skill presence ---
    if treatment_config.skill_dir:
        runner.skills_invoked.append("teamcity-cli")
        runner.passed("Skill loaded via treatment")

    runner.print_summary()

    # --- Log to Sentry (no-op if SENTRY_DSN unset) ---
    sentry_log.log_run(
        task_name=task_config.name,
        treatment_name=treatment_config.name,
        instruction=task_config.instruction,
        experiment_id=experiment_id,
        response_text=runner.text,
        pass_rate=runner.pass_rate,
        duration_sec=result.duration_sec,
        num_turns=events.num_turns,
        input_tokens=events.input_tokens,
        output_tokens=events.output_tokens,
        skill_invoked=bool(runner.skills_invoked),
        check_results=runner.summary()["results"],
        tool_calls=events.tool_calls,
        tool_results=events.tool_results,
        llm_grades=[
            {"dimension": g.dimension, "score": g.score, "reasoning": g.reasoning}
            for g in llm_grades
        ],
        run_id=run_id,
    )

    # --- Save artifacts ---
    artifacts_dir = EVALS_ROOT / "results" / experiment_id
    artifacts_dir.mkdir(parents=True, exist_ok=True)
    prefix = f"{task_config.name}_{treatment_config.name}_{run_id}"
    (artifacts_dir / f"{prefix}.raw.jsonl").write_text(result.raw_output)
    summary = runner.summary()
    summary["events"] = events.summary()
    summary["llm_grades"] = [
        {"dimension": g.dimension, "score": g.score, "reasoning": g.reasoning}
        for g in llm_grades
    ]
    (artifacts_dir / f"{prefix}.json").write_text(json.dumps(summary, indent=2))

    # --- Assert ---
    assert runner.pass_rate >= 0.5, (
        f"Pass rate {runner.pass_rate:.0%} below threshold 50% — "
        f"{runner.failed_count}/{runner.total_count} checks failed"
    )
