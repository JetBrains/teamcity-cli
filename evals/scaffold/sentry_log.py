"""Sentry logger — pushes eval results as AI-Agent traces.

A parent transaction (op=gen_ai.invoke_agent) wraps child spans (op=gen_ai.execute_tool),
one per Claude tool call. Tags (experiment_id/task/treatment) make results groupable
in Discover; measurements (pass_rate/duration/tokens/num_turns) are the numeric signal
compare.py aggregates across branches.

Safe to import even when SENTRY_DSN is unset — init becomes a no-op and logging is skipped.
"""

from __future__ import annotations

import json
import os
import time
from datetime import datetime, timedelta, timezone
from typing import Any

_INITIALIZED = False


def init_if_configured() -> bool:
    """Initialize Sentry once per process. Returns True if active."""
    global _INITIALIZED
    if _INITIALIZED:
        return True

    dsn = os.environ.get("SENTRY_DSN", "").strip()
    if not dsn:
        return False

    try:
        import sentry_sdk
    except ImportError:
        return False

    sentry_sdk.init(
        dsn=dsn,
        traces_sample_rate=1.0,
        send_default_pii=False,
        release=os.environ.get("SENTRY_RELEASE") or os.environ.get("BRANCH_NAME") or None,
        environment=os.environ.get("SENTRY_ENVIRONMENT", "eval"),
        max_breadcrumbs=0,
    )
    _INITIALIZED = True
    return True


def flush(timeout: float = 5.0) -> None:
    if not _INITIALIZED:
        return
    try:
        import sentry_sdk
        sentry_sdk.flush(timeout=timeout)
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
    llm_grades: list | None = None,
    run_id: str | None = None,
) -> None:
    """Push one eval run to Sentry as a transaction + child spans. No-op if DSN unset."""
    if not init_if_configured():
        return

    try:
        import sentry_sdk

        # Backdate the transaction so Sentry shows a trace duration that matches
        # Claude's actual runtime, not the ~ms it takes to create the spans.
        end = datetime.now(timezone.utc)
        start = end - timedelta(seconds=max(duration_sec, 0.0))

        txn = sentry_sdk.start_transaction(
            op="gen_ai.invoke_agent",
            name=f"{task_name}/{treatment_name}",
            start_timestamp=start,
            sampled=True,
        )
        try:
            txn.set_tag("experiment_id", experiment_id)
            txn.set_tag("task", task_name)
            txn.set_tag("treatment", treatment_name)
            txn.set_tag("skill_invoked", "true" if skill_invoked else "false")
            # Bucket pass_rate into tags so dashboards (spans dataset) can count by status.
            # Numeric aggregation still requires the deprecated transactions dataset via
            # measurements.pass_rate.
            txn.set_tag("passed", "true" if pass_rate >= 0.5 else "false")
            if pass_rate >= 1.0:
                bucket = "perfect"
            elif pass_rate >= 0.8:
                bucket = "good"
            elif pass_rate >= 0.5:
                bucket = "marginal"
            else:
                bucket = "fail"
            txn.set_tag("pass_bucket", bucket)
            if run_id:
                txn.set_tag("run_id", run_id)

            model = os.environ.get("BENCH_CC_MODEL", "")
            txn.set_data("gen_ai.system", "anthropic")
            txn.set_data("gen_ai.agent.name", "claude-code")
            txn.set_data("gen_ai.request.model", model)
            txn.set_data("gen_ai.response.model", model)
            txn.set_data("gen_ai.usage.input_tokens", input_tokens)
            txn.set_data("gen_ai.usage.output_tokens", output_tokens)
            txn.set_data("gen_ai.usage.total_tokens", input_tokens + output_tokens)
            txn.set_data("instruction", _truncate(instruction, 4000))
            txn.set_data("response", _truncate(response_text, 10000))
            txn.set_data("checks", _truncate(json.dumps(check_results), 8000))
            if llm_grades:
                txn.set_data("llm_grades", _truncate(json.dumps(llm_grades), 4000))

            # Measurements — queryable as `measurements.*` in the (deprecated) transactions
            # dataset. Capped at 10 per txn by Sentry.
            sentry_sdk.set_measurement("pass_rate", float(pass_rate))
            sentry_sdk.set_measurement("duration_sec", float(duration_sec), "second")
            sentry_sdk.set_measurement("num_turns", float(num_turns))
            sentry_sdk.set_measurement("total_tokens", float(input_tokens + output_tokens))


            # Timeline: alternate LLM-call spans and tool-call spans across the parent
            # duration. Claude's stream-json doesn't expose per-turn token splits, so
            # tokens are divided evenly across `num_turns` LLM spans. This populates
            # Sentry's AI Agents view ("LLM Calls by Model", "Tokens Used").
            total_slots = max(num_turns, 1) + len(tool_calls)
            slot_sec = max(duration_sec, 0.0) / total_slots
            cursor = start

            llm_per_call_in = input_tokens // max(num_turns, 1)
            llm_per_call_out = output_tokens // max(num_turns, 1)
            for i in range(max(num_turns, 1)):
                llm_start = cursor
                llm_end = llm_start + timedelta(seconds=slot_sec)
                try:
                    with txn.start_child(
                        op="gen_ai.chat",
                        name=f"chat {model}" if model else "chat",
                        start_timestamp=llm_start,
                    ) as span:
                        span.set_data("gen_ai.system", "anthropic")
                        span.set_data("gen_ai.operation.name", "chat")
                        span.set_data("gen_ai.request.model", model)
                        span.set_data("gen_ai.response.model", model)
                        span.set_data("gen_ai.usage.input_tokens", llm_per_call_in)
                        span.set_data("gen_ai.usage.output_tokens", llm_per_call_out)
                        span.set_data("gen_ai.usage.total_tokens", llm_per_call_in + llm_per_call_out)
                        span.finish(end_timestamp=llm_end)
                except Exception:
                    pass
                cursor = llm_end

            for tc in tool_calls:
                tool_start = cursor
                tool_end = tool_start + timedelta(seconds=slot_sec)
                try:
                    with txn.start_child(
                        op="gen_ai.execute_tool",
                        name=f"execute_tool {tc.get('name', '')}",
                        start_timestamp=tool_start,
                    ) as span:
                        span.set_data("gen_ai.tool.name", tc.get("name", ""))
                        span.set_data(
                            "gen_ai.tool.input",
                            _truncate(json.dumps(tc.get("input", {}), default=str), 6000),
                        )
                        output_text = _tool_output_text(tool_results.get(tc.get("id", "")))
                        if output_text:
                            span.set_data("gen_ai.tool.output", _truncate(output_text, 6000))
                        span.finish(end_timestamp=tool_end)
                except Exception:
                    pass
                cursor = tool_end
        finally:
            txn.finish(end_timestamp=end)
    except Exception as e:
        print(f"  [sentry] Warning: {e}")
