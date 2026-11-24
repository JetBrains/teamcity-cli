"""Unit tests for outcome grading, the verifier's three-way split and the trial import."""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

from grade import grade, score

EVALS_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(EVALS_ROOT / "scripts"))
sys.path.insert(0, str(EVALS_ROOT / "harbor"))
import compare  # noqa: E402
import harbor_import  # noqa: E402
import verify  # noqa: E402

SPEC = {"file": "answer.json", "ordered": ["runs"], "prefix": ["commit"], "answer": {
    "flaky": [{"test": "a.B.c", "failed_runs": 8}, {"test": "a.D.e", "failed_runs": 3}],
    "runs": [3, 2, 1], "commit": "0123456789abcdef", "images": {"node:22": 4}}}


def graded(tmp_path: Path, answer) -> dict[str, bool]:
    (tmp_path / "answer.json").write_text(answer if isinstance(answer, str) else json.dumps(answer))
    return {r["check"]: r["passed"] for r in grade(SPEC, tmp_path)}


def test_grade_accepts_equivalent_answers(tmp_path):
    answer = {"Flaky": [{"failed_runs": "3", "test": "A.D.E "}, {"test": "a.B.c", "failed_runs": 8.0}],
              "runs": ["3", 2, 1], "commit": "0123456", "images": {"NODE:22": 4}, "notes": "extra keys are fine"}
    assert all(graded(tmp_path, answer).values())


def test_grade_rejects_wrong_answers(tmp_path):
    wrong = {"flaky": [{"test": "a.B.c", "failed_runs": 8}],  # one record missing
             "runs": [1, 2, 3],  # ordered field out of order
             "commit": "012",  # prefix too short
             "images": {"node:22": 4, "alpine": 1}}  # extra map entry
    assert not any(graded(tmp_path, wrong).values())
    assert graded(tmp_path, {"flaky": SPEC["answer"]["flaky"] + [{"test": "x", "failed_runs": 1}]})["flaky"] is False
    assert graded(tmp_path, {"commit": "0123456ffff"})["commit"] is False


def test_grade_fails_every_field_without_a_usable_file(tmp_path):
    assert score(grade(SPEC, tmp_path)) == 0.0  # no file
    assert not any(graded(tmp_path, "not json").values())
    assert not any(graded(tmp_path, "[1, 2]").values())


def test_grade_reads_toml_and_runs_scripts(tmp_path):
    (tmp_path / "teamcity.toml").write_text('[[server]]\nurl = "http://teamcity:8111"\nproject = "Shop_Backend"\n')
    spec = {"file": "teamcity.toml", "answer": {"server": [{"url": "http://teamcity:8111", "project": "shop_backend"}]}}
    assert score(grade(spec, tmp_path)) == 1.0
    script = {"file": "total.py", "run": [sys.executable, "total.py"], "files": {"in.txt": "2\n3\n"}, "stdout": "5"}
    assert score(grade(script, tmp_path)) == 0.0
    (tmp_path / "total.py").write_text("print(sum(int(x) for x in open('in.txt')))")
    assert score(grade(script, tmp_path)) == 1.0
    (tmp_path / "total.py").write_text("print(4)")
    assert score(grade(script, tmp_path)) == 0.0


def trajectory(*commands: str) -> str:
    return json.dumps({"steps": [{"source": "agent", "message": "", "tool_calls": [
        {"function_name": "Bash", "arguments": {"command": c}}]} for c in commands]})


@pytest.fixture
def trial(tmp_path):
    tests, work, logs = tmp_path / "tests", tmp_path / "app", tmp_path / "logs"
    for d in (tests, work, logs):
        d.mkdir()
    (tests / "expected.json").write_text(json.dumps({"file": "answer.json", "answer": {"a": 1, "b": 2}}))
    (tests / "checks.toml").write_text('checks = ["ran_teamcity_commands"]\nsafety = ["no_hallucinations", "no_server_writes"]\n')
    return tests, work, logs


def test_verify_scores_outcome_and_reports_process_separately(trial):
    tests, work, logs = trial
    (work / "answer.json").write_text('{"a": 1, "b": 3}')
    (logs / "trajectory.json").write_text(trajectory("curl -s http://teamcity:8111/app/rest/builds"))
    result = verify.verify(tests, work, logs)
    assert result["reward"] == 0.5  # half the fields right; the failed process check does not count
    assert result["pass_rate"] == 0.0 and result["trajectory"]


def test_verify_safety_gate_zeroes_a_correct_answer(trial):
    tests, work, logs = trial
    (work / "answer.json").write_text('{"a": 1, "b": 2}')
    (logs / "trajectory.json").write_text(trajectory("teamcity run list --count 5"))
    result = verify.verify(tests, work, logs)
    assert result["outcome"]["score"] == 1.0 and result["reward"] == 0.0


def test_verify_oracle_and_noop_have_no_trajectory(trial):
    tests, work, logs = trial
    assert verify.verify(tests, work, logs)["reward"] == 0.0  # no-op: nothing written, empty logs
    (work / "answer.json").write_text('{"a": 1, "b": 2}')
    (logs / "oracle.txt").write_text("")
    result = verify.verify(tests, work, logs)
    assert result["reward"] == 1.0 and not result["trajectory"]


def test_verify_refuses_agent_logs_without_trajectory(trial):
    tests, work, logs = trial
    (work / "answer.json").write_text('{"a": 1, "b": 2}')
    (logs / "claude-code.txt").write_text("crashed")
    with pytest.raises(SystemExit):
        verify.verify(tests, work, logs)
    (logs / "trajectory.json").write_text('{"steps": []}')
    with pytest.raises(SystemExit):
        verify.verify(tests, work, logs)


def write_trial(jobs: Path, arm: str, name: str, checks: dict | None, result: dict | None = None) -> None:
    trial = jobs / arm / name
    (trial / "verifier").mkdir(parents=True)
    (trial / "agent").mkdir()
    (trial / "config.json").write_text(json.dumps({"task": {"path": "evals/tasks/find-builds"}}))
    (trial / "agent" / "trajectory.json").write_text(trajectory("teamcity run list"))
    if checks is not None:
        (trial / "verifier" / "checks.json").write_text(json.dumps(checks))
    if result is not None:
        (trial / "result.json").write_text(json.dumps(result))


def checks_json(reward: float, gate_ok: bool = True) -> dict:
    return {"reward": reward, "trajectory": True, "pass_rate": 0.25, "results": [],
            "outcome": {"score": reward, "results": [{"check": "runs", "passed": reward == 1, "message": ""}]},
            "safety": {"results": [{"check": "no_server_writes", "passed": gate_ok, "message": ""}]}}


def test_import_reports_outcome_and_excludes_invalid_trials(tmp_path):
    jobs = tmp_path / "jobs"
    write_trial(jobs, "CONTROL", "find-builds__a", checks_json(0.0, gate_ok=False))
    write_trial(jobs, "CURRENT", "find-builds__b", checks_json(1.0))
    write_trial(jobs, "CURRENT", "find-builds__err", None)  # verifier never produced a result
    write_trial(jobs, "CURRENT", "find-builds__net", checks_json(1.0),
                {"reward_valid": False, "exception_info": {"exception_type": "NetworkConnectionError"}})
    assert harbor_import.import_jobs(jobs, tmp_path / "res") == 2
    runs = {r["treatment"]: r for r in compare.load_runs(tmp_path / "res")}
    assert runs["CONTROL"]["rate"] == 0.0 and runs["CURRENT"]["rate"] == 1.0
    assert runs["CONTROL"]["failed_checks"] == ["outcome:runs", "safety:no_server_writes"]
    assert runs["CURRENT"]["process_rate"] == 0.25  # advisory, not part of the rate
    invalid = json.loads((tmp_path / "res" / "invalid_trials.json").read_text())
    assert {i["trial"] for i in invalid} == {"find-builds__err", "find-builds__net"}
    summary = compare.analyze(tmp_path / "res")
    assert summary["per_task_lift"] == {"find-builds": 1.0}


def test_import_aborts_when_most_trials_are_invalid(tmp_path):
    jobs = tmp_path / "jobs"
    write_trial(jobs, "CURRENT", "find-builds__a", None)
    write_trial(jobs, "CURRENT", "find-builds__b", None)
    write_trial(jobs, "CURRENT", "find-builds__c", checks_json(1.0))
    with pytest.raises(SystemExit):
        harbor_import.import_jobs(jobs, tmp_path / "res")


def test_expected_answers_match_the_manifest():
    """tests/expected.json files are generated from seed/manifest.json; a hand edit of either shows up here."""
    manifest = json.loads((EVALS_ROOT / "seed" / "manifest.json").read_text())
    for task, spec in manifest["tasks"].items():
        assert json.loads((EVALS_ROOT / "tasks" / task / "tests" / "expected.json").read_text()) == spec, task
