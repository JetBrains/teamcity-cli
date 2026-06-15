"""Langfuse logger — pushes eval results as traces with native scores.

One trace per eval run (name = task). A generation child carries token usage so
trace-level token metrics populate; one tool span per Claude tool call feeds the
"tool calls by tool" breakdown. Quality signals become first-class Langfuse
scores (pass_rate, passed, pass_bucket, judge:<dim>), so dashboards aggregate
them directly — no span-timing reconstruction like the Sentry path needs.
Langfuse is observability only — gating still runs on local artifacts via
scripts/compare.py.

Targets the Langfuse Python SDK v4 (`langfuse>=4`). Safe to import/run with no
keys set: init becomes a no-op and logging is skipped.
"""

from __future__ import annotations

import os
from typing import Any

_client: Any = None
_RESOLVED = False


def init_if_configured() -> Any:
    """Return a cached Langfuse client, or None when keys are unset."""
    global _client, _RESOLVED
    if _RESOLVED:
        return _client
    _RESOLVED = True

    if not (os.environ.get("LANGFUSE_PUBLIC_KEY") and os.environ.get("LANGFUSE_SECRET_KEY")):
        return None
    try:
        from langfuse import Langfuse
    except ImportError:
        return None

    _client = Langfuse(
        host=os.environ.get("LANGFUSE_HOST") or os.environ.get("LANGFUSE_BASE_URL") or None,
        environment=os.environ.get("LANGFUSE_TRACING_ENVIRONMENT")
        or os.environ.get("SENTRY_ENVIRONMENT", "eval"),
        release=os.environ.get("LANGFUSE_RELEASE") or os.environ.get("BRANCH_NAME") or None,
    )
    return _client


def flush(timeout: float = 10.0) -> None:
    if _client is not None:
        try:
            _client.flush()
        except Exception:
            pass


def _truncate(s: str, limit: int = 8000) -> str:
    return s if len(s) <= limit else s[:limit] + f"…[truncated {len(s) - limit}]"


def _tool_output_text(result_block: dict[str, Any] | None) -> str:
    if not isinstance(result_block, dict):
        return ""
    out = ""
    for block in result_block.get("content", []) or []:
        if isinstance(block, dict) and block.get("type") == "text":
            out += block.get("text", "")
    return out


def _bucket(pass_rate: float) -> str:
    if pass_rate >= 1.0:
        return "perfect"
    if pass_rate >= 0.8:
        return "good"
    if pass_rate >= 0.5:
        return "marginal"
    return "fail"


def log_run(
    *,
    task_name: str,
    treatment_name: str,
    instruction: str,
    experiment_id: str,
    response_text: str,
    pass_rate: float,
    duration_sec: float,
    num_turns: int,
    input_tokens: int,
    output_tokens: int,
    skill_invoked: bool,
    check_results: list[dict],
    tool_calls: list[dict],
    tool_results: dict[str, dict],
    skill_available: bool = False,
    llm_grades: list | None = None,
    ungraded_dimensions: list[str] | None = None,
    provenance: dict | None = None,
    run_id: str | None = None,
) -> None:
    """Push one eval run to Langfuse as a trace + scores. No-op if keys unset."""
    client = init_if_configured()
    if client is None:
        return

    try:
        import langfuse as _lf

        prov = provenance or {}
        model = prov.get("model") or os.environ.get("BENCH_CC_MODEL", "")
        total_tokens = input_tokens + output_tokens
        bucket = _bucket(pass_rate)
        instruction_t = _truncate(instruction, 4000)
        response_t = _truncate(response_text, 10000)

        # Tags fold the cross-tab dimensions into a flat, filterable space —
        # Langfuse dashboards group by one dimension, so e.g. treatment:CURRENT
        # is how a widget isolates one arm.
        tags = [
            f"task:{task_name}",
            f"treatment:{treatment_name}",
            f"skill_available:{str(skill_available).lower()}",
            f"pass_bucket:{bucket}",
        ]
        if model:
            tags.append(f"model:{model}")

        # Short string dimensions live at the trace level (propagate_attributes
        # requires str values); the rich payload rides on the root observation.
        trace_meta = {
            "task": task_name,
            "treatment": treatment_name,
            "experiment_id": experiment_id,
            "pass_bucket": bucket,
            "model": model,
            "skill_available": str(skill_available).lower(),
            "skill_invoked": str(skill_invoked).lower(),
        }
        if run_id:
            trace_meta["run_id"] = run_id

        obs_meta = {
            "num_turns": num_turns,
            "duration_sec": duration_sec,
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
            "total_tokens": total_tokens,
            "checks": check_results,
            "provenance": prov,
        }
        if ungraded_dimensions:
            obs_meta["ungraded_dimensions"] = ungraded_dimensions

        with _lf.propagate_attributes(
            trace_name=task_name,
            user_id=treatment_name,
            session_id=experiment_id,
            tags=tags,
            version=prov.get("git_sha"),
            metadata=trace_meta,
        ):
            with client.start_as_current_observation(
                as_type="agent",
                name=f"{task_name}/{treatment_name}",
                input=instruction_t,
                output=response_t,
                metadata=obs_meta,
            ):
                # Token rollup so trace-level token metrics aggregate in dashboards.
                client.start_observation(
                    as_type="generation",
                    name=f"chat {model}".strip() or "chat",
                    model=model or None,
                    usage_details={
                        "input": input_tokens,
                        "output": output_tokens,
                        "total": total_tokens,
                    },
                ).end()

                for tc in tool_calls:
                    out_text = _tool_output_text(tool_results.get(tc.get("id", "")))
                    client.start_observation(
                        as_type="tool",
                        name=tc.get("name", "tool"),
                        input=tc.get("input", {}),
                        output=_truncate(out_text, 6000) if out_text else None,
                    ).end()

                # Quality signals as native scores — the reason Langfuse fits
                # evals better than Sentry: these aggregate without measurement hacks.
                score = client.score_current_trace
                score(name="pass_rate", value=float(pass_rate), data_type="NUMERIC")
                score(name="passed", value=1 if pass_rate >= 0.5 else 0, data_type="BOOLEAN")
                score(name="pass_bucket", value=bucket, data_type="CATEGORICAL")
                score(name="skill_invoked", value=1 if skill_invoked else 0, data_type="BOOLEAN")
                score(name="duration_sec", value=float(duration_sec), data_type="NUMERIC")
                score(name="num_turns", value=float(num_turns), data_type="NUMERIC")
                score(name="total_tokens", value=float(total_tokens), data_type="NUMERIC")

                for g in llm_grades or []:
                    dim, sc = g.get("dimension"), g.get("score")
                    if dim is None or sc is None:
                        continue
                    score(
                        name=f"judge:{dim}",
                        value=float(sc),
                        data_type="NUMERIC",
                        comment=_truncate(str(g.get("reasoning", "")), 500),
                    )
    except Exception as e:
        print(f"  [langfuse] Warning: {e}")
