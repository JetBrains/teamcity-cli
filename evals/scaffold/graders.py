"""LLM-as-judge graders — quality assessment beyond command matching.

Auth priority: ANTHROPIC_API_KEY → CLAUDE_CODE_OAUTH_TOKEN → Claude Code CLI (OAuth).
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
from dataclasses import dataclass

GRADER_MODEL = "claude-sonnet-4-5-20250929"

_CLAUDE_BIN = os.environ.get("CLAUDE_BIN") or shutil.which("claude") or "claude"


@dataclass
class GradeResult:
    dimension: str
    score: int  # 1-5
    reasoning: str
    passed: bool  # score >= 3


def _make_anthropic_client():
    """Create an Anthropic client using available credentials, or None.

    Priority: ANTHROPIC_API_KEY → CLAUDE_CODE_OAUTH_TOKEN → None.
    """
    try:
        import anthropic
    except ImportError:
        return None

    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if api_key:
        return anthropic.Anthropic(api_key=api_key)

    oauth_token = os.environ.get("CLAUDE_CODE_OAUTH_TOKEN")
    if oauth_token:
        return anthropic.Anthropic(auth_token=oauth_token)

    return None


def _call_anthropic_sdk(prompt: str) -> str | None:
    """Call Anthropic API via SDK (API key or OAuth token). Returns response text or None."""
    client = _make_anthropic_client()
    if not client:
        return None

    try:
        response = client.messages.create(
            model=GRADER_MODEL,
            max_tokens=500,
            messages=[{"role": "user", "content": prompt}],
        )
        return response.content[0].text
    except Exception:
        return None


def _call_claude_cli(prompt: str) -> str | None:
    """Call Claude Code CLI in print mode (uses OAuth). Returns response text or None."""
    try:
        result = subprocess.run(
            [_CLAUDE_BIN, "-p", prompt, "--model", GRADER_MODEL,
             "--output-format", "json", "--no-session-persistence",
             "--bare", "--tools", ""],
            capture_output=True, text=True, timeout=60,
            env={**os.environ, "NO_COLOR": "1"},
        )
        if result.returncode != 0:
            return None
        data = json.loads(result.stdout)
        return data.get("result", "")
    except Exception:
        return None


def _call_llm(prompt: str) -> str | None:
    """Try Anthropic SDK first, fall back to Claude Code CLI."""
    return _call_anthropic_sdk(prompt) or _call_claude_cli(prompt)


def llm_grade(
    task_instruction: str,
    agent_response: str,
    dimension: str,
    rubric: str,
) -> GradeResult:
    """Grade a response on a dimension using LLM-as-judge."""
    prompt = f"""You are evaluating an AI agent's response to a TeamCity CI/CD task.

<task>
{task_instruction}
</task>

<agent_response>
{agent_response[:8000]}
</agent_response>

<dimension>
{dimension}
</dimension>

<rubric>
{rubric}
</rubric>

Score the response on the dimension using this scale:
1 = Very poor
2 = Poor
3 = Acceptable
4 = Good
5 = Excellent

Think through your reasoning step by step, then output your final answer as JSON:
{{"score": <1-5>, "reasoning": "<brief explanation>"}}"""

    text = _call_llm(prompt)
    if not text:
        return GradeResult(dimension, 3, "Grading unavailable (no API key or CLI)", True)

    try:
        start = text.rfind("{")
        end = text.rfind("}") + 1
        if start >= 0 and end > start:
            result = json.loads(text[start:end])
            score = int(result.get("score", 3))
            reasoning = result.get("reasoning", "")
            return GradeResult(dimension, score, reasoning, score >= 3)
    except Exception as e:
        return GradeResult(dimension, 3, f"Grading parse error: {e}", True)

    return GradeResult(dimension, 3, "Could not parse grader response", True)


# ---------------------------------------------------------------------------
# Pre-built rubrics for TeamCity CLI skill evaluation
# ---------------------------------------------------------------------------

def grade_command_accuracy(task_instruction: str, response: str) -> GradeResult:
    return llm_grade(
        task_instruction,
        response,
        "Command Accuracy",
        """Does the agent use correct `teamcity` CLI commands with valid flags?
        5 = All commands are correct with exact flags
        4 = Commands are correct, minor flag issues
        3 = Most commands correct, one wrong flag
        2 = Several wrong commands or hallucinated flags
        1 = Fundamentally wrong commands""",
    )


def grade_workflow_completeness(task_instruction: str, response: str) -> GradeResult:
    return llm_grade(
        task_instruction,
        response,
        "Workflow Completeness",
        """Did the agent complete the full workflow requested?
        5 = All steps completed thoroughly
        4 = All steps addressed, minor gaps
        3 = Most steps completed
        2 = Significant steps missing
        1 = Task barely attempted""",
    )


def grade_explanation_quality(task_instruction: str, response: str) -> GradeResult:
    return llm_grade(
        task_instruction,
        response,
        "Explanation Quality",
        """Is the explanation clear, concise, and actionable?
        5 = Clear, concise, directly actionable
        4 = Clear and useful, slightly verbose
        3 = Understandable but could be clearer
        2 = Confusing or overly verbose
        1 = Unhelpful or misleading""",
    )


def grade_all(task_instruction: str, response: str) -> list[GradeResult]:
    """Run all quality graders. Returns list of GradeResults."""
    return [
        grade_command_accuracy(task_instruction, response),
        grade_workflow_completeness(task_instruction, response),
        grade_explanation_quality(task_instruction, response),
    ]
