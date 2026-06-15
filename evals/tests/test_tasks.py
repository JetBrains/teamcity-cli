"""Main eval test runner.

Pytest pass/fail means HARNESS HEALTH only — a CONTROL run scoring zero checks
is a measurement, not a test failure. Quality gating happens in aggregate over
the written artifacts (scripts/compare.py: paired skill lift + CI).

Usage:
    just eval                                    # all tasks, default treatments
    just eval --task=investigate-failure          # single task
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
from scaffold import langfuse_log, sentry_log
from scaffold.tasks import TaskConfig
from conftest import TreatmentConfig
from checks import CHECK_REGISTRY

EVALS_ROOT = Path(__file__).resolve().parent.parent
if str(EVALS_ROOT) not in sys.path:
    sys.path.insert(0, str(EVALS_ROOT))


@pytest.mark.timeout(900)
@pytest.mark.usefixtures("verify_env", "verify_skill")
def test_task(
    task_config: TaskConfig,
    treatment_config: TreatmentConfig,
    experiment_id: str,
    run_id: str,
    provenance: dict,
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

    # --- Run deterministic checks from CHECK_REGISTRY ---
    runner = EvalRunner(events, task_name=f"{task_config.name}/{treatment_config.name}")
    check_fns = [CHECK_REGISTRY[c] for c in task_config.checks]
    runner.run(check_fns)

    # --- LLM grading: advisory, reported separately, fails closed ---
    llm_grades = {}
    if task_config.llm_grade and os.environ.get("ANTHROPIC_API_KEY") and runner.text:
        llm_grades = grade_all(task_config.instruction, runner.text)
    graded = [
        {"dimension": dim, "score": g.score, "reasoning": g.reasoning}
        for dim, g in llm_grades.items() if g is not None
    ]
    ungraded = [dim for dim, g in llm_grades.items() if g is None]

    # --- Skill usage: availability is the treatment; invocation is observed ---
    skill_available = treatment_config.skill_dir is not None
    skill_invoked = any("teamcity" in s.lower() for s in events.skills_invoked)

    runner.print_summary()
    for g in graded:
        print(f"  [llm] {g['dimension']}: {g['score']}/5 — {g['reasoning']}")
    if ungraded:
        print(f"  [llm] ungraded (judge unavailable): {', '.join(ungraded)}")

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
        skill_available=skill_available,
        skill_invoked=skill_invoked,
        check_results=runner.summary()["results"],
        tool_calls=events.tool_calls,
        tool_results=events.tool_results,
        llm_grades=graded,
        ungraded_dimensions=ungraded,
        provenance=provenance,
        run_id=run_id,
    )

    # --- Log to Langfuse (no-op if keys unset) ---
    langfuse_log.log_run(
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
        skill_available=skill_available,
        skill_invoked=skill_invoked,
        check_results=runner.summary()["results"],
        tool_calls=events.tool_calls,
        tool_results=events.tool_results,
        llm_grades=graded,
        ungraded_dimensions=ungraded,
        provenance=provenance,
        run_id=run_id,
    )

    # --- Save artifacts ---
    artifacts_dir = EVALS_ROOT / "results" / experiment_id
    artifacts_dir.mkdir(parents=True, exist_ok=True)
    prefix = f"{task_config.name}_{treatment_config.name}_{run_id}"
    (artifacts_dir / f"{prefix}.raw.jsonl").write_text(result.raw_output)
    summary = runner.summary()
    summary["task_id"] = task_config.name
    summary["treatment"] = treatment_config.name
    summary["skill_available"] = skill_available
    summary["skill_invoked"] = skill_invoked
    summary["events"] = events.summary()
    summary["llm_grades"] = graded
    summary["ungraded_dimensions"] = ungraded
    summary["provenance"] = provenance
    (artifacts_dir / f"{prefix}.json").write_text(json.dumps(summary, indent=2))

    # --- Assert: harness sanity only ---
    # An agent timeout is a (poor) quality outcome already captured by the
    # checks; anything else that produced no parseable events is a harness bug.
    if result.exit_code != 124:
        assert result.exit_code == 0, (
            f"harness: claude exited {result.exit_code}: …{result.raw_output[-500:]}"
        )
        assert events.num_turns > 0, "harness: no events parsed from claude output"
