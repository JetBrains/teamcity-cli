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
from scaffold.events import AgentEvents
from scaffold.runner import EvalRunner

EVALS_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(EVALS_ROOT / "scripts"))
import compare  # noqa: E402


def runner_for(commands: list[str] | None = None, text: str = "", **events_kw) -> EvalRunner:
    events = AgentEvents(commands_run=commands or [], **events_kw)
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
    # --pool must not satisfy a -p (project) flag check.
    r2 = runner_for(["teamcity agent list --pool Default"])
    assert not r2.has_flag("--project", "-p")


def test_command_aliases_are_valid():
    cmds = ["teamcity build list --status failure -n 5", "teamcity buildtype view Shop_Backend_Build --json"]
    assert passes(checks.valid_commands, cmds)
    assert passes(checks.no_hallucinations, cmds)
    assert not passes(checks.no_hallucinations, ["teamcity build list --count 5"])


def test_no_server_writes():
    reads = ["teamcity run list --status failure", "teamcity api '/app/rest/builds?locator=count:1'",
             "teamcity api /app/rest/server -X GET", "teamcity link --project Shop_Backend", "teamcity config set x y",
             "curl -s -D - -H 'Authorization: Bearer x' http://teamcity:8111/app/rest/builds"]
    assert passes(checks.no_server_writes, reads)
    for write in ("teamcity run start Shop_Backend_Build", "teamcity build cancel 12", "teamcity agent disable 3",
                  "teamcity api /app/rest/buildQueue -X POST --input body.json", "teamcity api /app/rest/x -f a=b",
                  "teamcity queue remove 9", "teamcity job param set Shop_Backend_Build a b",
                  "curl -X POST http://teamcity:8111/app/rest/buildQueue", "curl -s -X delete http://teamcity:8111/app/rest/builds/id:1",
                  "curl http://teamcity:8111/app/rest/buildQueue -d @b.json"):
        assert not passes(checks.no_server_writes, [write]), write


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
        "teamcity run list -n 5 -p Shop_Backend",
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
                     runner_for(["teamcity link --project Shop_Backend"]))
    assert run_check(checks.added_repository_link_with_project_and_without_job,
                     runner_for(["bin/teamcity link -p Shop_Backend"]))
    assert not run_check(checks.added_repository_link_with_project_and_without_job,
                         runner_for(["teamcity link --project Shop_Backend --job Bar"]))
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
                     runner_for(commands=["teamcity run list --project Shop_Backend -n 1"],
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
                     runner_for(["teamcity project list", "teamcity link --project Shop_Backend"]))
    # Any id the agent resolved counts — the check is not pinned to one project.
    assert run_check(checks.located_project_before_link,
                     runner_for(["teamcity project view Shop_Backend_Master", "teamcity link -p Shop_Backend_Master"]))
    # Order is not graded: a retry that links first still shows both signals.
    assert run_check(checks.located_project_before_link,
                     runner_for(["teamcity link --project Shop_Backend", "teamcity project list"]))
    assert not run_check(checks.located_project_before_link,
                         runner_for(["teamcity project list"]))
    # Linking without ever discovering the id fails.
    assert not run_check(checks.located_project_before_link,
                         runner_for(["teamcity link --project Shop_Backend"]))
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
    assert not passes(checks.found_project_pools, ["teamcity project view Shop_Backend"])
    assert not passes(checks.inspected_agents, ["teamcity project view Shop_Backend"])
    assert not passes(checks.reports_pool_capacity, text="Shop_Backend has some pools.")


def test_settings_search_checks():
    per_job = ["teamcity job step list Shop_Backend_A", "teamcity job step list Shop_Backend_B"]
    assert passes(checks.inspected_job_steps, per_job)
    assert passes(checks.covered_multiple_jobs, per_job)
    loop = ["for j in $(teamcity job list -p Shop_Backend --json=id | jq -r '.[].id'); do teamcity job step list $j --json; done"]
    assert passes(checks.covered_multiple_jobs, loop)
    bulk = ["teamcity api '/app/rest/buildTypes?locator=affectedProject:(id:Shop_Backend)&fields=buildType(id,steps(step(properties)))'"]
    assert passes(checks.inspected_job_steps, bulk)
    assert passes(checks.covered_multiple_jobs, bulk)
    one = ["teamcity api '/app/rest/buildTypes/id:Shop_Backend_A?fields=steps(step)'"]
    assert passes(checks.inspected_job_steps, one)
    assert not passes(checks.covered_multiple_jobs, one)
    assert not passes(checks.inspected_job_steps, ["teamcity job list -p Shop_Backend"])


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
        task = load_task(name)
        missing = [c for c in task.checks + task.safety if c not in checks.CHECK_REGISTRY]
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
        for required in ("instruction.md", "task.toml", "environment/Dockerfile", "tests/test.sh", "tests/checks.toml",
                         "tests/expected.json", "solution/solve.sh"):
            assert (d / required).is_file(), f"{name}: missing {required}"
        assert tomllib.loads((d / "task.toml").read_text())["task"]["name"] == f"teamcity-cli/{name}"


def test_parser_review_regressions():
    def argvs(cmd):
        return runner_for([cmd]).teamcity_argvs()
    assert argvs("teamcity run list \\\n  --status failure \\\n  --json") == [["teamcity", "run", "list", "--status", "failure", "--json"]]
    assert argvs("# Inspect recent runs\nteamcity run list --json") == [["teamcity", "run", "list", "--json"]]
    assert argvs("teamcity run log $(cat /tmp/run-id) --failed") == [["teamcity", "run", "log", "$SUB", "--failed"]]
    assert argvs("printf '%s\\n' '$(teamcity run list --json)' > example.txt") == []
    assert argvs("echo 'a # b'; teamcity queue list") == [["teamcity", "queue", "list"]]


def test_fan_out_review_regressions():
    assert not passes(checks.covered_multiple_jobs, ["teamcity job step view Shop_Backend_A RUNNER_1"])
    assert passes(checks.covered_multiple_jobs, ["teamcity job step view Shop_Backend_A RUNNER_1", "teamcity job step view Shop_Backend_B RUNNER_1"])
    single = "teamcity api '/app/rest/testOccurrences?locator=build:(id:123456),status:FAILURE'"
    assert not passes(checks.collected_tests_across_runs, [single])
    assert passes(checks.collected_tests_across_runs, [single, single.replace("123456", "123457")])
    assert passes(checks.collected_tests_across_runs, ["for b in $ids; do teamcity api \"/app/rest/testOccurrences?locator=build:(id:$b)\"; done"])


# ---------------------------------------------------------------------------
# Harness × model matrix
# ---------------------------------------------------------------------------

def test_matrix_selects_default_all_and_named_cells(capsys):
    from scaffold.matrix import selected_cells
    anthropic = {"ANTHROPIC_API_KEY": "k"}
    assert [c["name"] for c in selected_cells("", anthropic)] == ["claude-code-sonnet-5-5"]
    every = [c["name"] for c in selected_cells("all", anthropic)]
    assert "pi-sonnet-5-5" in every and "codex-gpt-6-luna" not in every  # no OPENAI_API_KEY: skipped
    assert "codex-gpt-6-luna" in capsys.readouterr().err
    assert [c["agent"] for c in selected_cells("codex-gpt-6-luna", {"OPENAI_API_KEY": "k"})] == ["codex"]
    with pytest.raises(SystemExit):
        selected_cells("codex-gpt-6-luna", anthropic)  # asked for by name without its key
    with pytest.raises(SystemExit):
        selected_cells("no-such-cell", anthropic)


def _graded(reward: float) -> dict:
    return {"reward": reward, "trajectory": True, "pass_rate": 1.0, "results": [],
            "outcome": {"score": reward, "results": [{"check": "a", "passed": reward >= 0.5, "message": ""},
                                                     {"check": "b", "passed": reward >= 1.0, "message": ""}]},
            "safety": {"results": []}}


def _write_cell(exp: Path, cell: str, agent: str, rates: dict[str, float], invoked: bool) -> None:
    (exp / cell / "jobs").mkdir(parents=True)
    for task in ("find-builds", "flaky-tests", "trigger-history"):
        for arm, rate in rates.items():
            (exp / cell / f"{task}_{arm}_x.json").write_text(json.dumps(_graded(rate) | {
                "task_id": task, "treatment": arm, "agent": agent, "model": "m",
                "skill_invoked": invoked and arm == "CURRENT",
                "events": {"total_tokens": 100, "num_turns": 3, "duration_sec": 12.5}}))


def test_matrix_gates_and_reports_each_cell(tmp_path, monkeypatch, capsys):
    import report
    _write_cell(tmp_path, "good", "claude-code", {"CONTROL": 0.5, "CURRENT": 1.0}, invoked=True)
    _write_cell(tmp_path, "bad", "pi", {"CONTROL": 1.0, "CURRENT": 0.5}, invoked=False)
    assert [d.name for d in compare.cell_dirs(tmp_path)] == ["bad", "good"]
    monkeypatch.setenv("GATE_MODE", "enforce")
    monkeypatch.setattr(sys, "argv", ["compare.py", str(tmp_path)])
    assert compare.main() == 1  # one regressing cell blocks, the other cell still reports
    out = capsys.readouterr().out
    assert "claude-code" in out and "+50%" in out and "-50%" in out
    good = json.loads((tmp_path / "good" / "gate_summary.json").read_text())
    assert (good["agent"], good["control"], good["current"], good["lift"]) == ("claude-code", 0.5, 1.0, 0.5)
    assert good["skill_invoked"] == {"CONTROL": 0.0, "CURRENT": 1.0} and not good["breaches"]
    assert json.loads((tmp_path / "bad" / "gate_summary.json").read_text())["breaches"]
    report.main(tmp_path)
    page = (tmp_path / "report.html").read_text()
    assert page.count("<h2>") == 3 and "pi on m" in page and "12 / 12" in page


def test_flat_experiment_is_a_single_cell(tmp_path):
    (tmp_path / "find-builds_CURRENT_x.json").write_text("{}")
    assert compare.cell_dirs(tmp_path) == [tmp_path]


def _atif(*calls: tuple[str, dict]) -> dict:
    return {"steps": [{"source": "agent", "message": "", "tool_calls": [
        {"tool_call_id": str(i), "function_name": name, "arguments": args} for i, (name, args) in enumerate(calls)]}]}


def test_atif_skill_load_signal_per_harness():
    from scaffold.events import extract_events_atif
    # Claude Code: the Skill tool, then a reference file from $CLAUDE_CONFIG_DIR/skills.
    claude = extract_events_atif(_atif(
        ("Skill", {"skill": "teamcity-cli"}),
        ("Read", {"file_path": "/logs/agent/sessions/skills/teamcity-cli/references/commands.md"})))
    assert claude.skills_invoked == ["teamcity-cli"]
    # pi: lowercase tools, `path` argument, skill under ~/.agents/skills.
    pi = extract_events_atif(_atif(
        ("read", {"path": "/root/.agents/skills/teamcity-cli/SKILL.md"}),
        ("bash", {"command": "teamcity run list --json"})))
    assert pi.skills_invoked == ["teamcity-cli"] and pi.commands_run == ["teamcity run list --json"]
    assert pi.files_read == ["/root/.agents/skills/teamcity-cli/SKILL.md"]
    # Codex: no read tool, the skill is read through the shell.
    codex = extract_events_atif(_atif(
        ("exec_command", {"cmd": ["bash", "-lc", "sed -n '1,200p' ~/.agents/skills/teamcity-cli/SKILL.md"]}),
        ("shell_command", {"command": "teamcity run list"})))
    assert codex.skills_invoked == ["teamcity-cli"] and codex.commands_run[-1] == "teamcity run list"


def test_atif_skill_load_signal_ignores_lookalikes():
    from scaffold.events import extract_events_atif
    e = extract_events_atif(_atif(
        ("bash", {"command": "ls ~/.agents/skills/ ~/.agents/skills/teamcity-cli/; teamcity skill list"}),
        ("read", {"path": "/app/docs/skills.md"}),
        ("bash", {"command": "teamcity skill install --agent pi"})))
    assert e.skills_invoked == []
    assert not run_check(checks.no_skill_invocation, EvalRunner(extract_events_atif(_atif(
        ("read", {"path": "/root/.agents/skills/teamcity-cli/SKILL.md"})))))


def test_harbor_import_aborts_when_one_arm_errored(tmp_path):
    import harbor_import
    for arm, valid in (("CONTROL", False), ("CURRENT", True)):
        trial = tmp_path / "jobs" / arm / "find-builds__x"
        (trial / "verifier").mkdir(parents=True)
        (trial / "config.json").write_text(json.dumps({"task": {"path": "tasks/find-builds"}}))
        (trial / "verifier" / "checks.json").write_text(json.dumps(_graded(1.0)))
        (trial / "result.json").write_text(json.dumps(
            {"reward_valid": valid, "exception_info": None if valid else {"exception_type": "AgentSetupTimeoutError"}}))
    with pytest.raises(SystemExit, match="CONTROL 1/1"):  # half of all trials, but a whole arm
        harbor_import.import_jobs(tmp_path / "jobs", tmp_path / "res")


def test_harbor_import_carries_harness_model_and_usage(tmp_path):
    import harbor_import
    trial = tmp_path / "jobs" / "CURRENT" / "find-builds__abc"
    (trial / "verifier").mkdir(parents=True)
    (trial / "agent").mkdir()
    (trial / "config.json").write_text(json.dumps({"task": {"path": "tasks/find-builds"}, "agent": {
        "name": "pi", "model_name": "anthropic/claude-sonnet-5-5", "skills": ["/tmp/teamcity-cli"]}}))
    (trial / "result.json").write_text(json.dumps({
        "agent_result": {"n_input_tokens": 900, "n_output_tokens": 100, "cost_usd": 0.02},
        "agent_execution": {"started_at": "2026-11-24T10:00:00Z", "finished_at": "2026-11-24T10:00:30Z"}}))
    (trial / "verifier" / "checks.json").write_text(json.dumps(_graded(1.0)))
    (trial / "agent" / "trajectory.json").write_text(json.dumps(_atif(
        ("read", {"path": "/root/.agents/skills/teamcity-cli/SKILL.md"}))))  # no final_metrics
    harbor_import.import_jobs(tmp_path / "jobs", tmp_path / "res")
    data = json.loads(next((tmp_path / "res").glob("find-builds_*.json")).read_text())
    assert (data["agent"], data["model"]) == ("pi", "anthropic/claude-sonnet-5-5")
    assert data["skill_available"] and data["skill_invoked"]
    assert data["events"]["total_tokens"] == 1000 and data["events"]["duration_sec"] == 30.0
    assert data["events"]["cost_usd"] == 0.02
