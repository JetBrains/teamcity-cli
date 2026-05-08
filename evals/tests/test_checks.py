from typing import Callable

from checks import (
    avoids_raw_maven_for_dsl_validation,
    uses_project_settings_validate,
    validates_with_explicit_dot_path,
)
from scaffold.events import ClaudeEvents
from scaffold.runner import EvalRunner


def _run_check_and_get_result(commands: list[str], check_fn: Callable[[EvalRunner], None]) -> dict:
    events = ClaudeEvents(commands_run=commands)
    runner = EvalRunner(events, task_name="unit")
    runner.run([check_fn])
    return runner.summary()["results"][0]


def test_uses_project_settings_validate_passes() -> None:
    result = _run_check_and_get_result(
        ["teamcity project settings validate . --verbose"],
        uses_project_settings_validate,
    )
    assert result["passed"] is True


def test_validates_with_explicit_dot_path_fails_without_dot_path() -> None:
    result = _run_check_and_get_result(
        ["teamcity project settings validate --verbose"],
        validates_with_explicit_dot_path,
    )
    assert result["passed"] is False


def test_avoids_raw_maven_for_dsl_validation_fails_on_maven() -> None:
    result = _run_check_and_get_result(
        ["teamcity project settings validate . --verbose", "mvn -q -DskipTests package"],
        avoids_raw_maven_for_dsl_validation,
    )
    assert result["passed"] is False
