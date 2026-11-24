"""Unit tests for the eval harness itself — no API calls, no live server.

Adversarial fixtures: known-bad agent behavior must FAIL the checks and
known-good behavior must PASS them, so the suite can't silently rot into
always-green.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

import checks
from scaffold.events import ClaudeEvents
from scaffold.runner import EvalRunner

EVALS_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(EVALS_ROOT / "scripts"))
import compare  # noqa: E402


def runner_for(commands: list[str] | None = None, text: str = "", **events_kw) -> EvalRunner:
    events = ClaudeEvents(commands_run=commands or [], **events_kw)
    if text:
        events.assistant_messages.append(text)
    return EvalRunner(events, task_name="unit")


def run_check(check, runner: EvalRunner) -> bool:
    runner.run([check])
    assert runner.total_count == 1
    return runner._results[-1].passed


# ---------------------------------------------------------------------------
# Command parsing
# ---------------------------------------------------------------------------

def test_argv_parsing_handles_pipes_and_env_prefix():
    r = runner_for([
        "teamcity run list --json | jq '.[].id'",
        "TEAMCITY_URL=https://x.example teamcity queue list",
        "bin/teamcity pool list",
    ])
    argvs = r.teamcity_argvs()
    assert len(argvs) == 3
    assert r.has_subcommand("run", "list")
    assert r.has_subcommand("queue", "list")
    assert r.has_subcommand("pool", "list")
    assert r.has_flag("--json")


def test_argv_parsing_sees_loops_and_substitutions():
    r = runner_for([
        "for id in 1 2; do teamcity run tests $id --failed; done",
        'B=$(teamcity api "/app/rest/builds?locator=count:1&fields=build(id)" | jq .)',
        "if teamcity auth status\nthen teamcity queue list\nfi",
        "echo `teamcity pool list --json`",
        "timeout 90 teamcity agent list -n 5",
        "env -u TEAMCITY_TOKEN teamcity project list",
        "diff <(teamcity pipeline pull P) local.yml",
        'echo "== $p: $(teamcity job list -p $p | wc -l)"',
    ])
    assert r.has_subcommand("agent", "list")
    assert r.has_subcommand("project", "list")
    assert r.has_subcommand("pipeline", "pull")
    assert r.has_subcommand("job", "list")
    assert r.has_subcommand("run", "tests")
    assert r.has_subcommand("api")
    assert r.has_subcommand("auth", "status")
    assert r.has_subcommand("queue", "list")
    assert r.has_subcommand("pool", "list")
    assert r.has_flag("--failed")


def test_argv_parsing_keeps_quoted_operators_and_drops_redirections():
    r = runner_for([
        'teamcity api "/app/rest/builds?locator=count:1&fields=build(id)" 2>/dev/null | jq .',
        "teamcity run log 123456 > out.txt",
        "teamcity run tests 654321 &>/dev/null",
        "teamcity run list --json '' | head",
    ])
    assert r.teamcity_argvs() == [
        ["teamcity", "api", "/app/rest/builds?locator=count:1&fields=build(id)"],
        ["teamcity", "run", "log", "123456"],
        ["teamcity", "run", "tests", "654321"],
        ["teamcity", "run", "list", "--json", ""],
    ]


def test_heredoc_body_is_not_executed():
    r = runner_for(["cat > /tmp/w.sh <<'EOF' | head -1\nteamcity run start X --watch\nEOF\nteamcity queue list"])
    assert r.has_subcommand("queue", "list")
    assert not r.has_subcommand("run", "start")


def test_non_teamcity_commands_are_not_invocations():
    r = runner_for(["grep teamcity notes.txt", "echo 'teamcity build'"])
    assert r.teamcity_argvs() == []
    assert run_check(checks.no_teamcity_commands, r)


def test_substring_traps_do_not_trip_tokenized_checks():
    # Project IDs containing "build" used to fail uses_run_not_build.
    r = runner_for(["teamcity run list --project IjPlatformBuild --status failure"])
    assert run_check(checks.uses_run_not_build, r)
    # --pool must not satisfy a -p (project) flag check.
    r2 = runner_for(["teamcity agent list --pool Default"])
    assert not r2.has_flag("--project", "-p")


def test_uses_run_not_build_catches_wrong_noun():
    r = runner_for(["teamcity build list"])
    assert not run_check(checks.uses_run_not_build, r)


# ---------------------------------------------------------------------------
# Schema-driven validity (against the real generated cli_schema.json)
# ---------------------------------------------------------------------------

def test_valid_commands_accepts_real_tree():
    r = runner_for([
        "teamcity run tree 123",          # missing from the old hand-list
        "teamcity pipeline list",          # whole group missing from the old hand-list
        "teamcity job settings get X",
        "teamcity --version",
        "teamcity help run",
    ])
    assert run_check(checks.valid_commands, r)


def test_valid_commands_rejects_unknown():
    r = runner_for(["teamcity runs list"])
    assert not run_check(checks.valid_commands, r)


def test_valid_commands_rejects_invented_subcommand_of_real_group():
    for cmd in ("teamcity run frobnicate 12345", "teamcity project madeup"):
        assert not run_check(checks.valid_commands, runner_for([cmd]))


def test_valid_commands_accepts_positionals_after_leaf():
    r = runner_for(["teamcity run log 12345 --failed"])
    assert run_check(checks.valid_commands, r)


def test_no_hallucinations_accepts_real_flags():
    # --tail/--follow are real `run log` flags the old blocklist punished.
    r = runner_for([
        "teamcity run log 123 --tail 50",
        "teamcity run log 123 --follow",
        "teamcity run list --status failure --limit 10 --json",
        "teamcity run list -n 5 -p JBR",
        "teamcity run list --help",
    ])
    assert run_check(checks.no_hallucinations, r)


def test_no_hallucinations_rejects_invented_flags():
    for cmd in [
        "teamcity run list --count 10",
        "teamcity run list --format json",
        "teamcity run log 123 --errors",
        "teamcity run tests 123 --sort duration",
    ]:
        r = runner_for([cmd])
        assert not run_check(checks.no_hallucinations, r), cmd


# ---------------------------------------------------------------------------
# Previously broken guardrails
# ---------------------------------------------------------------------------

def test_acknowledges_limitation_can_fail():
    r = runner_for(text="Here are the test results sorted by duration.")
    assert not run_check(checks.acknowledges_limitation, r)
    r2 = runner_for(text="The CLI doesn't support sorting; pipe through jq instead.")
    assert run_check(checks.acknowledges_limitation, r2)


def test_produces_python_detects_headless_file_write():
    r = runner_for(files_created=["sales_report.py"])
    assert run_check(checks.produces_python, r)
    r2 = runner_for(commands=["python3 total.py"])
    assert run_check(checks.produces_python, r2)
    r3 = runner_for(text="I summed the amount column by hand: 42.")
    assert not run_check(checks.produces_python, r3)


def test_runner_resets_current_check_after_run():
    r = runner_for(["teamcity run list"])
    r.run([checks.lists_builds])
    assert r._current_check == ""


# ---------------------------------------------------------------------------
# repository binding
# ---------------------------------------------------------------------------

def test_checked_repository_link():
    assert run_check(checks.checked_repository_link, runner_for(["cat teamcity.toml"]))
    # Fails cleanly on real signal — not a masked AttributeError (has_command bug).
    r = runner_for(["teamcity run list"])
    assert not run_check(checks.checked_repository_link, r)
    assert "Exception" not in r._results[-1].message


def test_added_repository_link():
    # Grades the attempt, not its exit status — a binding call is a pass.
    assert run_check(checks.added_repository_link, runner_for(["teamcity link --project Foo"]))
    # Usage-only (bare / --help) is not a binding attempt.
    assert not run_check(checks.added_repository_link, runner_for(["teamcity link --help"]))


def test_added_repository_link_with_project_and_without_job():
    assert run_check(checks.added_repository_link_with_project_and_without_job,
                     runner_for(["teamcity link --project JBR"]))
    assert run_check(checks.added_repository_link_with_project_and_without_job,
                     runner_for(["bin/teamcity link -p JBR"]))
    assert not run_check(checks.added_repository_link_with_project_and_without_job,
                         runner_for(["teamcity link --project JBR --job Bar"]))
    # Linking a different project than requested must not earn the check.
    assert not run_check(checks.added_repository_link_with_project_and_without_job,
                         runner_for(["teamcity link --project Foo"]))


def test_mentioned_teamcity_toml():
    assert run_check(checks.mentioned_teamcity_toml,
                     runner_for(text="I updated teamcity.toml with the binding."))
    assert not run_check(checks.mentioned_teamcity_toml,
                         runner_for(text="I ran the link command."))


def test_did_not_add_repository_link():
    assert run_check(checks.did_not_add_repository_link, runner_for(["teamcity run list"]))
    # Any link touch is off-task drift — including help-only.
    assert not run_check(checks.did_not_add_repository_link, runner_for(["teamcity link --help"]))


def test_did_not_modify_teamcity_toml():
    assert run_check(checks.did_not_modify_teamcity_toml, runner_for(["cat teamcity.toml"]))
    # files_modified path (Edit tool)
    assert not run_check(checks.did_not_modify_teamcity_toml,
                         runner_for(files_modified=["/tmp/teamcity.toml"]))
    # files_created path (Write tool)
    assert not run_check(checks.did_not_modify_teamcity_toml,
                         runner_for(files_created=["/repo/teamcity.toml"]))
    # shell mutation path — redirect, tee, in-place sed, removal
    assert not run_check(checks.did_not_modify_teamcity_toml,
                         runner_for(["echo '[server]' > teamcity.toml"]))
    assert not run_check(checks.did_not_modify_teamcity_toml,
                         runner_for(["cat <<EOF > /tmp/work/teamcity.toml"]))
    assert not run_check(checks.did_not_modify_teamcity_toml,
                         runner_for(["tee teamcity.toml < config.txt"]))
    assert not run_check(checks.did_not_modify_teamcity_toml,
                         runner_for(["sed -i 's/old/new/' teamcity.toml"]))
    assert not run_check(checks.did_not_modify_teamcity_toml,
                         runner_for(["rm -f teamcity.toml"]))
    # Innocent words ending in "rm", and read-only reads, must not trip it.
    assert run_check(checks.did_not_modify_teamcity_toml,
                     runner_for(["cat teamcity.toml # confirm the binding"]))
    assert run_check(checks.did_not_modify_teamcity_toml,
                     runner_for(["cat teamcity.toml 2>/dev/null", "rm -rf /tmp/scratch"]))


def test_used_project_from_repository_link():
    # Must inspect teamcity.toml AND query within the linked project — via shell...
    assert run_check(checks.used_project_from_repository_link,
                     runner_for(["cat teamcity.toml", "teamcity run list -n 1"]))
    # ...or via a file read, with the linked project named explicitly.
    assert run_check(checks.used_project_from_repository_link,
                     runner_for(commands=["teamcity run list --project JBR -n 1"],
                                files_read=["/repo/teamcity.toml"]))
    # A no-skill agent that never consults the binding fails — even though the
    # bare query is auto-scoped by the CLI. This is what isolates the skill.
    assert not run_check(checks.used_project_from_repository_link,
                         runner_for(["teamcity run list -n 1"]))
    # Inspected, but overrode to a different project → fail.
    assert not run_check(checks.used_project_from_repository_link,
                         runner_for(["cat teamcity.toml", "teamcity run list --project SomethingElse"]))
    # Inspected, but never ran a build query → fail.
    assert not run_check(checks.used_project_from_repository_link,
                         runner_for(["cat teamcity.toml", "teamcity project list"]))


def test_located_project_before_link():
    assert run_check(checks.located_project_before_link,
                     runner_for(["teamcity project list", "teamcity link --project JBR"]))
    # Any id the agent resolved counts — the check is not pinned to one project.
    assert run_check(checks.located_project_before_link,
                     runner_for(["teamcity project view JBR_Master", "teamcity link -p JBR_Master"]))
    # Order is not graded: a retry that links first still shows both signals.
    assert run_check(checks.located_project_before_link,
                     runner_for(["teamcity link --project JBR", "teamcity project list"]))
    assert not run_check(checks.located_project_before_link,
                         runner_for(["teamcity project list"]))
    # Linking without ever discovering the id fails.
    assert not run_check(checks.located_project_before_link,
                         runner_for(["teamcity link --project JBR"]))
    assert not run_check(checks.located_project_before_link,
                         runner_for(["teamcity project list", "teamcity link --help"]))
    # --auto binds from git remotes, not the located project → not "located then linked".
    assert not run_check(checks.located_project_before_link,
                         runner_for(["teamcity project list", "teamcity link --auto"]))


# ---------------------------------------------------------------------------
# Fan-out tasks
# ---------------------------------------------------------------------------

def test_scoped_to_task_jobs():
    assert run_check(checks.scoped_to_flaky_job, runner_for([f"teamcity run list --job {checks.FLAKY_JOB} -n 20"]))
    assert run_check(checks.scoped_to_trigger_job, runner_for([f"teamcity run list -j {checks.TRIGGER_JOB}"]))
    assert not run_check(checks.scoped_to_flaky_job, runner_for(["teamcity run list -n 10"]))
    assert not run_check(checks.scoped_to_trigger_job, runner_for([f"teamcity run list -j {checks.FLAKY_JOB}"]))


def test_collected_tests_across_runs():
    good = [
        ["teamcity run tests 123456 --failed", "teamcity run tests 123457 --failed"],
        ["for id in $(teamcity run list -j X -n 20 --json=id | jq -r '.[].id'); do teamcity run tests $id --failed; done"],
        ['teamcity api "/app/rest/testOccurrences?locator=build:(buildType:X,count:20),status:FAILURE"'],
        ["teamcity run tests --job X --test com.acme.FooTest"],
    ]
    for cmds in good:
        assert run_check(checks.collected_tests_across_runs, runner_for(cmds)), cmds
    # one run, even when its --limit value looks like a second id
    assert not run_check(checks.collected_tests_across_runs, runner_for(["teamcity run tests 123456 --limit 5000"]))
    assert not run_check(checks.collected_tests_across_runs, runner_for(["teamcity run list -j X"]))


def test_reports_failure_counts():
    assert run_check(checks.reports_failure_counts, runner_for(text="FooTest.bar failed in 7/20 runs"))
    assert run_check(checks.reports_failure_counts, runner_for(text="- BarTest: 3 runs"))
    assert not run_check(checks.reports_failure_counts, runner_for(text="Several tests look flaky."))


def passes(check, commands=None, text=""):
    return run_check(check, runner_for(commands, text))


def test_why_queued_checks():
    cmds = ["teamcity queue list --json", "teamcity run view 998877"]
    text = "It is waiting because there are no compatible agents: 0 agents match."
    assert passes(checks.inspected_queue, cmds)
    assert passes(checks.inspected_queued_run, cmds)
    assert passes(checks.explains_wait_reason, text=text)
    api = ['teamcity api "/app/rest/buildQueue/id:998877?fields=waitReason,compatibleAgents(count)"']
    assert passes(checks.inspected_queue, api)
    assert passes(checks.inspected_queued_run, api)
    assert not passes(checks.inspected_queued_run, ["teamcity queue list"])
    assert not passes(checks.explains_wait_reason, text="The queue is long.")
    assert not passes(checks.inspected_queue, ["teamcity run list"])


def test_pool_capacity_checks():
    cmds = ["teamcity pool list", "teamcity agent list --pool 3 --json"]
    assert passes(checks.found_project_pools, cmds)
    assert passes(checks.inspected_agents, cmds)
    assert passes(checks.reports_pool_capacity, text="Pool Linux: 12 connected, 9 busy, all Linux.")
    api = ['teamcity api "/app/rest/agentPools?fields=agentPool(id,name)"',
           'teamcity api "/app/rest/agents?locator=pool:(id:3)"']
    assert passes(checks.found_project_pools, api)
    assert passes(checks.inspected_agents, api)
    assert not passes(checks.found_project_pools, ["teamcity project view JBR"])
    assert not passes(checks.inspected_agents, ["teamcity project view JBR"])
    assert not passes(checks.reports_pool_capacity, text="JBR has some pools.")


def test_settings_search_checks():
    per_job = ["teamcity job step list JBR_A", "teamcity job step list JBR_B"]
    assert passes(checks.inspected_job_steps, per_job)
    assert passes(checks.covered_multiple_jobs, per_job)
    loop = ["for j in $(teamcity job list -p JBR --json=id | jq -r '.[].id'); do teamcity job step list $j --json; done"]
    assert passes(checks.covered_multiple_jobs, loop)
    bulk = ["teamcity api '/app/rest/buildTypes?locator=affectedProject:(id:JBR)&fields=buildType(id,steps(step(properties)))'"]
    assert passes(checks.inspected_job_steps, bulk)
    assert passes(checks.covered_multiple_jobs, bulk)
    one = ["teamcity api '/app/rest/buildTypes/id:JBR_A?fields=steps(step)'"]
    assert passes(checks.inspected_job_steps, one)
    assert not passes(checks.covered_multiple_jobs, one)
    assert not passes(checks.inspected_job_steps, ["teamcity job list -p JBR"])


def test_trigger_history_checks():
    assert passes(checks.read_trigger_info, [f"teamcity run list --job {checks.TRIGGER_JOB} -n 30 --json"])
    assert not passes(checks.read_trigger_info, ["teamcity run list --job X -n 10"])
    table = "| Source | Runs |\n| VCS change | 7 |\n| schedule | 3 |"
    assert passes(checks.summarizes_trigger_sources, text=table)
    # a source word and a number on different lines is not a summary
    assert not passes(checks.summarizes_trigger_sources, text="Most were started by a user.\nI looked at 10 runs.")


def test_tasks_reference_registered_checks():
    from scaffold.tasks import list_tasks, load_task
    for name in list_tasks():
        missing = [c for c in load_task(name).checks if c not in checks.CHECK_REGISTRY]
        assert not missing, f"{name}: {missing}"


# ---------------------------------------------------------------------------
# Gate statistics
# ---------------------------------------------------------------------------

def test_pass_k_estimator():
    assert compare.pass_k([True, True], k=2) == 1.0
    assert compare.pass_k([True, False], k=2) == 0.0
    assert compare.pass_k([True], k=2) is None
    assert compare.pass_k([True, True, False], k=2) == pytest.approx(1 / 3)


def test_bootstrap_ci_brackets_constant_lift():
    by_task = {
        f"t{i}": {"CONTROL": [0.5, 0.5], "CURRENT": [0.8, 0.8]} for i in range(6)
    }
    low, high = compare.bootstrap_lift_ci(by_task, list(by_task))
    assert low == pytest.approx(0.3)
    assert high == pytest.approx(0.3)


def test_bootstrap_ci_widens_with_noise():
    by_task = {
        "a": {"CONTROL": [0.2, 0.8], "CURRENT": [0.5, 1.0]},
        "b": {"CONTROL": [0.0, 0.4], "CURRENT": [0.9, 0.3]},
        "c": {"CONTROL": [0.6, 0.6], "CURRENT": [0.6, 0.6]},
        "d": {"CONTROL": [0.1, 0.9], "CURRENT": [0.9, 0.1]},
    }
    low, high = compare.bootstrap_lift_ci(by_task, list(by_task))
    assert low < high
    lifts = compare.paired_lifts(by_task)
    assert low <= sum(lifts.values()) / len(lifts) <= high


def test_efficiency_summary_averages_per_arm():
    runs = [
        {"treatment": "CONTROL", "events": {"total_tokens": 1000, "num_turns": 10,
                                            "tool_calls_count": 20}},
        {"treatment": "CONTROL", "events": {"total_tokens": 2000, "num_turns": 12,
                                            "tool_calls_count": 22}},
        {"treatment": "CURRENT", "events": {"total_tokens": 500, "num_turns": 6,
                                            "tool_calls_count": 8}},
    ]
    eff = compare.efficiency_summary(runs)
    assert eff["total_tokens"] == {"CONTROL": 1500, "CURRENT": 500}
    assert eff["num_turns"] == {"CONTROL": 11.0, "CURRENT": 6.0}


def test_efficiency_summary_tolerates_missing_and_malformed_events():
    # Legacy artifacts have no "events"; a partial one must not invent zeros for
    # metrics it never recorded, and non-numeric values must be ignored.
    runs = [
        {"treatment": "CONTROL", "events": {}},
        {"treatment": "CURRENT", "events": {"num_turns": 4}},
        {"treatment": "CURRENT", "events": {"num_turns": "n/a", "total_tokens": None}},
    ]
    eff = compare.efficiency_summary(runs)
    assert "total_tokens" not in eff
    assert eff["num_turns"] == {"CURRENT": 4.0}
    assert compare.efficiency_summary([{"treatment": "CONTROL", "events": {}}]) == {}


def test_load_runs_sanitizes_legacy_artifacts(tmp_path):
    legacy = {
        "task": "find-builds/CURRENT",
        "results": [
            {"check": "uses_json", "passed": True, "message": "Uses --json"},
            {"check": "no_hallucinations", "passed": False,
             "message": "[LLM] Command Accuracy: 2/5 — judge entry under wrong name"},
            {"check": "no_hallucinations", "passed": True,
             "message": "Skill loaded via treatment"},
            {"check": "uses_run_not_build", "passed": False, "message": "Used 'teamcity build'"},
        ],
    }
    (tmp_path / "find-builds_CURRENT_abc.json").write_text(json.dumps(legacy))
    runs = compare.load_runs(tmp_path)
    assert len(runs) == 1
    assert runs[0]["task"] == "find-builds"
    assert runs[0]["treatment"] == "CURRENT"
    assert runs[0]["rate"] == 0.5  # freebie and judge entries stripped


def test_guardrail_breach_blocks(tmp_path, monkeypatch):
    for i, ok in enumerate([False, False]):
        (tmp_path / f"negative-unrelated_CURRENT_{i}.json").write_text(json.dumps({
            "task_id": "negative-unrelated",
            "treatment": "CURRENT",
            "results": [{"check": "no_skill_invocation", "passed": ok, "message": ""}],
        }))
    summary = compare.analyze(tmp_path)
    assert any("guardrail" in b for b in summary["breaches"])
    monkeypatch.setenv("GATE_MODE", "enforce")
    assert compare.gate(summary) == 1
    monkeypatch.setenv("GATE_MODE", "warn")
    assert compare.gate(summary) == 0


def test_events_sum_metrics_across_result_messages():
    from scaffold.events import extract_events
    raw = "\n".join(json.dumps(o) for o in [
        {"type": "result", "num_turns": 21, "duration_ms": 246000, "usage": {"input_tokens": 900, "output_tokens": 300}},
        {"type": "system", "subtype": "init"},
        {"type": "result", "num_turns": 1, "duration_ms": 5000, "usage": {"input_tokens": 10, "output_tokens": 20}},
    ])
    e = extract_events(raw)
    assert (e.num_turns, e.duration_ms, e.input_tokens, e.output_tokens) == (22, 251000, 910, 320)


def test_fan_out_inside_generated_script_counts():
    heredoc = ("cat > /tmp/a.sh <<'EOF'\nfor id in $ids; do\n  teamcity run tests $id --failed --json\ndone\nEOF\n"
               "bash /tmp/a.sh")
    assert passes(checks.collected_tests_across_runs, [heredoc])
    r = runner_for(["bash ./a.sh"])
    r.events.tool_calls.append({"name": "Write", "input": {"file_path": "a.sh",
                                "content": "for j in $jobs; do teamcity job step list $j; done"}})
    assert run_check(checks.covered_multiple_jobs, r)
    nested = "cat > /tmp/c.sh <<'EOF'\nfor b in $ids; do\n  echo \"$(teamcity run tests \"$b\" --failed)\"\ndone\nEOF"
    assert passes(checks.collected_tests_across_runs, [nested])
    # a script mentioning one run is still a single run
    single = "cat > /tmp/b.sh <<'EOF'\nteamcity run tests 123456 --failed\nEOF\nbash /tmp/b.sh"
    assert not passes(checks.collected_tests_across_runs, [single])


def test_config_auth_keeps_token_out_of_env(tmp_path, monkeypatch):
    from scaffold import claude
    monkeypatch.setenv("TEAMCITY_URL", "https://tc.example")
    monkeypatch.setenv("TEAMCITY_TOKEN", "secret-token")
    monkeypatch.setenv("BENCH_AUTH", "config")
    from types import SimpleNamespace
    claude._setup_workspace(tmp_path, SimpleNamespace(name="CONTROL", skill_dir=None))
    env = claude._build_isolated_env(tmp_path)
    assert "TEAMCITY_TOKEN" not in env and "TEAMCITY_URL" not in env
    cfg = (tmp_path / ".config" / "tc" / "config.yml").read_text()
    assert "default_server: https://tc.example" in cfg and "token: secret-token" in cfg
    monkeypatch.delenv("BENCH_AUTH")
    assert claude._build_isolated_env(tmp_path)["TEAMCITY_TOKEN"] == "secret-token"


def test_atif_trajectory_feeds_the_same_checks():
    from scaffold.events import extract_events_atif
    trajectory = {"steps": [
        {"step_id": 1, "source": "user", "message": "find failed builds"},
        {"step_id": 2, "source": "agent", "message": "Listing runs.", "tool_calls": [
            {"tool_call_id": "a", "function_name": "Bash", "arguments": {"command": "teamcity run list --status failure --json"}}]},
        {"step_id": 3, "source": "agent", "message": "", "tool_calls": [
            {"tool_call_id": "b", "function_name": "exec_command", "arguments": {"cmd": ["bash", "-lc", "teamcity run log 123456 --failed"]}}]},
        {"step_id": 4, "source": "agent", "message": [{"type": "text", "text": "Run 123456 failed in step Compile."}]},
    ], "final_metrics": {"total_prompt_tokens": 900, "total_completion_tokens": 100}}
    e = extract_events_atif(trajectory)
    assert e.commands_run == ["teamcity run list --status failure --json", "teamcity run log 123456 --failed"]
    assert e.num_turns == 3 and e.input_tokens == 900
    r = EvalRunner(e)
    assert r.has_subcommand("run", "log") and r.has_flag("--failed")
    assert "Compile" in r.text


def test_harbor_tasks_are_well_formed():
    import tomllib
    from scaffold.tasks import TASKS_DIR, list_tasks
    for name in list_tasks():
        d = TASKS_DIR / name
        for required in ("instruction.md", "task.toml", "environment/Dockerfile", "tests/test.sh", "tests/checks.toml"):
            assert (d / required).is_file(), f"{name}: missing {required}"
        assert tomllib.loads((d / "task.toml").read_text())["task"]["name"] == f"teamcity-cli/{name}"
