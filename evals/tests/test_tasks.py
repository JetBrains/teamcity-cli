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
from scaffold.tasks import TaskConfig
from conftest import TreatmentConfig
from checks import CHECK_REGISTRY

EVALS_ROOT = Path(__file__).resolve().parent.parent
if str(EVALS_ROOT) not in sys.path:
    sys.path.insert(0, str(EVALS_ROOT))


def _log_to_langsmith(
    task: TaskConfig,
    treatment: TreatmentConfig,
    runner: EvalRunner,
    events,
    result,
    llm_grades: list,
    experiment_id: str,
) -> None:
    try:
        import uuid
        from langsmith import Client

        client = Client()
        project = os.environ.get("LANGSMITH_PROJECT", "teamcity-cli")
        parent_id = uuid.uuid4()

        client.create_run(
            id=parent_id,
            name=f"{task.name}/{treatment.name}",
            run_type="chain",
            project_name=project,
            inputs={
                "task": task.name,
                "treatment": treatment.name,
                "instruction": task.instruction,
            },
            outputs={
                "response": runner.text[:10000],
                "pass_rate": runner.pass_rate,
                "checks": runner.summary()["results"],
                "events": events.summary(),
            },
            extra={"metadata": {"experiment_id": experiment_id}},
        )

        for tc in events.tool_calls:
            tool_output = events.tool_results.get(tc["id"], {})
            output_text = ""
            if isinstance(tool_output, dict):
                for block in tool_output.get("content", []):
                    if isinstance(block, dict) and block.get("type") == "text":
                        output_text += block.get("text", "")[:3000]
            client.create_run(
                id=uuid.uuid4(),
                parent_run_id=parent_id,
                name=tc["name"],
                run_type="tool",
                project_name=project,
                inputs=tc.get("input", {}),
                outputs={"result": output_text[:5000]} if output_text else {},
            )

        client.create_run(
            id=uuid.uuid4(),
            parent_run_id=parent_id,
            name="validation",
            run_type="chain",
            project_name=project,
            inputs={"checks_count": runner.total_count},
            outputs={
                "pass_rate": runner.pass_rate,
                "results": runner.summary()["results"],
                "llm_grades": [
                    {"dimension": g.dimension, "score": g.score, "reasoning": g.reasoning}
                    for g in llm_grades
                ],
            },
        )

        feedback = {
            "checks_pass_rate": runner.pass_rate,
            "duration_seconds": result.duration_sec,
            "num_turns": float(events.num_turns),
            "skill_invoked": 1.0 if runner.skills_invoked else 0.0,
        }
        if events.input_tokens:
            feedback["total_tokens"] = float(events.input_tokens + events.output_tokens)
            feedback["command_count"] = float(len(events.commands_run))
        if llm_grades:
            feedback["avg_llm_score"] = sum(g.score for g in llm_grades) / len(llm_grades)
        for key, score in feedback.items():
            client.create_feedback(run_id=parent_id, key=key, score=score)
    except Exception as e:
        print(f"  [langsmith] Warning: {e}")


@pytest.mark.langsmith(test_suite_name="teamcity-cli-skill-eval")
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

    # --- Log to LangSmith ---
    _log_to_langsmith(task_config, treatment_config, runner, events, result, llm_grades, experiment_id)

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
