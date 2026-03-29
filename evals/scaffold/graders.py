"""LLM-as-judge graders — quality assessment beyond command matching.

Uses Claude to evaluate subjective dimensions of agent responses.
"""

from __future__ import annotations

import json
import os
from dataclasses import dataclass

GRADER_MODEL = "claude-sonnet-4-5-20250929"


@dataclass
class GradeResult:
    dimension: str
    score: int  # 1-5
    reasoning: str
    passed: bool  # score >= 3


def llm_grade(
    task_instruction: str,
    agent_response: str,
    dimension: str,
    rubric: str,
) -> GradeResult:
    """Grade a response on a dimension using LLM-as-judge.

    Returns a GradeResult with a 1-5 score and reasoning.
    Falls back to a neutral result if API is unavailable.
    """
    try:
        import anthropic
    except ImportError:
        return GradeResult(dimension, 3, "anthropic SDK not installed", True)

    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        return GradeResult(dimension, 3, "No ANTHROPIC_API_KEY", True)

    client = anthropic.Anthropic(api_key=api_key)

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

    try:
        response = client.messages.create(
            model=GRADER_MODEL,
            max_tokens=500,
            messages=[{"role": "user", "content": prompt}],
        )
        text = response.content[0].text

        # Extract JSON from response
        start = text.rfind("{")
        end = text.rfind("}") + 1
        if start >= 0 and end > start:
            result = json.loads(text[start:end])
            score = int(result.get("score", 3))
            reasoning = result.get("reasoning", "")
            return GradeResult(dimension, score, reasoning, score >= 3)
    except Exception as e:
        return GradeResult(dimension, 3, f"Grading error: {e}", True)

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
