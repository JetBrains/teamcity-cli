"""CHECK_REGISTRY — all validation functions in one place.

Each check takes an EvalRunner and calls runner.passed() or runner.failed().
Checks are referenced by ID in tasks.json.

Command validity is verified against `cli_schema.json`, generated from the
binary's real cobra tree (`go run scripts/generate-cli-schema.go`), so the
allowlist can never rot out of sync with the CLI.
"""

from __future__ import annotations

import json
import re
from functools import cache
from pathlib import Path

from scaffold.runner import EvalRunner

SCHEMA_PATH = Path(__file__).resolve().parent / "cli_schema.json"


@cache
def cli_schema() -> dict[str, list[str]]:
    """Command path → valid flags, generated from the binary's cobra tree."""
    if not SCHEMA_PATH.exists():
        raise FileNotFoundError(
            f"{SCHEMA_PATH} missing — run "
            "`go run scripts/generate-cli-schema.go > evals/cli_schema.json`"
        )
    return json.loads(SCHEMA_PATH.read_text())["commands"]


def resolve_command(argv: list[str]) -> tuple[str | None, list[str]]:
    """Longest schema-known command path for an argv, plus its valid flags.

    Returns (None, []) when the first subcommand token is unknown.
    Remaining non-flag tokens are positional arguments, not validated.
    """
    schema = cli_schema()
    tokens = EvalRunner.subcommand_tokens(argv)
    if not tokens:
        return "", schema[""]
    path = ""
    for tok in tokens:
        candidate = f"{path} {tok}".strip()
        if candidate not in schema:
            break
        path = candidate
    if not path:
        return None, []
    return path, schema[path]


# ---------------------------------------------------------------------------
# Shared: command validity and hallucination detection
# ---------------------------------------------------------------------------

def ran_teamcity_commands(runner: EvalRunner) -> None:
    """Claude must have actually executed teamcity commands, not just talked about them."""
    n = len(runner.teamcity_argvs())
    if n:
        runner.passed(f"Executed {n} teamcity command(s)")
    else:
        runner.failed("Did not execute any teamcity commands")


def no_auth_failure(runner: EvalRunner) -> None:
    """Fail if Claude got stuck on authentication instead of completing the task."""
    text = runner.text.lower()
    auth_phrases = ["authentication failed", "not authenticated", "unauthorized",
                    "401", "please provide", "need a token", "need credentials",
                    "i don't have access", "i cannot access", "authentication issue",
                    "could not authenticate"]
    if any(p in text for p in auth_phrases) and not runner.teamcity_argvs():
        runner.failed("Got stuck on authentication without completing the task")
    else:
        runner.passed("No auth failure blocking task completion")


def valid_commands(runner: EvalRunner) -> None:
    """Every executed subcommand path must exist in the binary's command tree."""
    invalid = []
    for argv in runner.teamcity_argvs():
        path, _ = resolve_command(argv)
        if path is None:
            invalid.append(" ".join(argv[:3]))
    if invalid:
        runner.failed(f"Unknown commands: {'; '.join(invalid)}")
    else:
        runner.passed("All commands are valid")


def no_hallucinations(runner: EvalRunner) -> None:
    """Every flag must exist on its command per the binary's real flag set."""
    violations = []
    for argv in runner.teamcity_argvs():
        path, allowed = resolve_command(argv)
        if path is None:
            continue  # valid_commands already reports unknown commands
        for tok in EvalRunner.flag_tokens(argv):
            name = tok.split("=", 1)[0]
            if name not in allowed:
                violations.append(f"'{name}' on 'teamcity {path}'".strip())
    if violations:
        runner.failed(f"Hallucinated flags: {'; '.join(violations)}")
    else:
        runner.passed("No hallucinated flags")


# ---------------------------------------------------------------------------
# investigate-failure
# ---------------------------------------------------------------------------

def found_failed_build(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "list") and runner.has_text("failure"):
        runner.passed("Found failed builds")
    elif runner.has_subcommand("run", "view"):
        runner.passed("Inspected a build")
    else:
        runner.failed("Did not search for builds")


def viewed_log(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "log"):
        runner.passed("Checked build log")
    else:
        runner.failed("Missing 'run log'")


def viewed_tests(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "tests"):
        runner.passed("Checked test results")
    else:
        runner.failed("Missing 'run tests'")


def viewed_changes(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "changes"):
        runner.passed("Checked changes")
    else:
        runner.failed("Missing 'run changes'")


def produced_diagnosis(runner: EvalRunner) -> None:
    text = runner.text.lower()
    if any(w in text for w in ["failed", "error", "exception", "cause", "problem", "broken", "timeout"]):
        runner.passed("Provided diagnosis")
    else:
        runner.failed("No diagnosis provided")


# ---------------------------------------------------------------------------
# daily-loop
# ---------------------------------------------------------------------------

def lists_builds(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "list"):
        runner.passed("Lists builds")
    else:
        runner.failed("Missing 'run list'")


def investigates_failure(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "log") and runner.has_subcommand("run", "tests"):
        runner.passed("Checks log and tests")
    elif runner.has_subcommand("run", "log") or runner.has_subcommand("run", "tests"):
        runner.passed("Checks log or tests")
    else:
        runner.failed("Did not investigate failure details")


def views_changes(runner: EvalRunner) -> None:
    viewed_changes(runner)


def handles_running(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "watch"):
        runner.passed("Uses 'run watch'")
    elif runner.has_text("watch") or runner.has_text("running") or runner.has_text("in progress"):
        runner.passed("Addresses running builds")
    else:
        runner.failed("Did not handle running builds")


def multi_step(runner: EvalRunner) -> None:
    n = len(runner.events.commands_run)
    if n >= 3:
        runner.passed(f"Multi-step workflow ({n} commands)")
    elif n >= 1:
        runner.passed(f"Executed {n} command(s)")
    else:
        runner.failed("No commands executed")


def no_thrashing(runner: EvalRunner) -> None:
    """Flag command thrashing — same command run more than twice."""
    from collections import Counter
    counts = Counter(runner.events.commands_run)
    repeated = {cmd: n for cmd, n in counts.items() if n > 2}
    if repeated:
        runner.failed(f"Command thrashing: {repeated}")
    else:
        runner.passed("No command thrashing")


# ---------------------------------------------------------------------------
# composite-failure
# ---------------------------------------------------------------------------

def finds_composite(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "list") or runner.has_subcommand("run", "view"):
        runner.passed("Finds builds")
    else:
        runner.failed("Did not search for builds")


def explores_dependencies(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "tree") or runner.has_subcommand("job", "tree"):
        runner.passed("Inspects the dependency tree")
    elif runner.has_subcommand("run", "list") and runner.has_flag("--status"):
        runner.passed("Filters by failure status")
    elif runner.has_subcommand("api"):
        runner.passed("Uses API for dependencies")
    else:
        runner.failed("Did not explore dependency chain")


def drills_into_child(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "log") or runner.has_subcommand("run", "tests"):
        runner.passed("Inspects child build details")
    else:
        runner.failed("Did not inspect child build")


def provides_diagnosis(runner: EvalRunner) -> None:
    produced_diagnosis(runner)


# ---------------------------------------------------------------------------
# inspect-url
# ---------------------------------------------------------------------------

def extracts_config_id(runner: EvalRunner) -> None:
    config_id = "ijplatform_master_CIDR_CLion_CLionTrunkHeavyTests_TestsUbuntu2404x86_64"
    in_command = any(
        config_id in tok for argv in runner.teamcity_argvs() for tok in argv
    )
    if in_command or runner.has_text(config_id):
        runner.passed("Extracted config ID from URL")
    else:
        runner.failed("Did not extract config ID from URL")


def lists_config_builds(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "list"):
        runner.passed("Listed builds for the configuration")
    elif runner.has_subcommand("run", "view"):
        runner.passed("Viewed build details")
    else:
        runner.failed("Did not list or view builds")


def provides_answer(runner: EvalRunner) -> None:
    if any(w in runner.text.lower() for w in ["clion", "test", "fail", "ubuntu", "docker"]):
        runner.passed("Provided build info")
    else:
        runner.failed("No useful info provided")


def executes_commands(runner: EvalRunner) -> None:
    n = len(runner.events.commands_run)
    if n >= 2:
        runner.passed(f"Executed {n} commands")
    elif n >= 1:
        runner.passed(f"Executed {n} command")
    else:
        runner.failed("No commands executed")


# ---------------------------------------------------------------------------
# find-builds
# ---------------------------------------------------------------------------

def uses_run_list(runner: EvalRunner) -> None:
    lists_builds(runner)


def filters_by_status(runner: EvalRunner) -> None:
    if runner.has_flag("--status"):
        runner.passed("Filters by status")
    else:
        runner.failed("Missing --status")


def filters_by_project(runner: EvalRunner) -> None:
    if runner.has_flag("--project", "-p"):
        runner.passed("Filters by project")
    else:
        runner.failed("Missing --project")


def uses_json(runner: EvalRunner) -> None:
    if runner.has_flag("--json"):
        runner.passed("Uses --json")
    else:
        runner.failed("Missing --json")


def uses_run_not_build(runner: EvalRunner) -> None:
    if runner.has_subcommand("build"):
        runner.failed("Used 'teamcity build' — should be 'teamcity run'")
    else:
        runner.passed("Correct: uses 'run' not 'build'")


# ---------------------------------------------------------------------------
# cross-project
# ---------------------------------------------------------------------------

def finds_subprojects(runner: EvalRunner) -> None:
    has_list = runner.has_subcommand("project", "list") or runner.has_subcommand("project", "tree")
    has_jcef = runner.has_text("JCEF") or runner.has_text("jcef") or runner.has_text("JBR_JCEF")
    if has_list and has_jcef:
        runner.passed("Found JCEF subprojects")
    elif has_list:
        runner.passed("Listed projects")
    else:
        runner.failed("Did not find subprojects")


def lists_jobs(runner: EvalRunner) -> None:
    if runner.has_subcommand("job", "list"):
        runner.passed("Lists jobs")
    else:
        runner.failed("Missing 'job list'")


def views_build_history(runner: EvalRunner) -> None:
    lists_builds(runner)


def checks_queue(runner: EvalRunner) -> None:
    if runner.has_subcommand("queue", "list"):
        runner.passed("Checks queue")
    else:
        runner.failed("Missing 'queue list'")


def provides_health_summary(runner: EvalRunner) -> None:
    if any(w in runner.text.lower() for w in ["success", "failure", "green", "red", "healthy", "failing", "stable", "flaky"]):
        runner.passed("Provides health assessment")
    else:
        runner.failed("No health summary")


# ---------------------------------------------------------------------------
# explore-infrastructure
# ---------------------------------------------------------------------------

def lists_projects(runner: EvalRunner) -> None:
    if runner.has_subcommand("project", "list"):
        runner.passed("Lists projects")
    else:
        runner.failed("Missing 'project list'")


def lists_pools(runner: EvalRunner) -> None:
    if runner.has_subcommand("pool", "list"):
        runner.passed("Lists pools")
    else:
        runner.failed("Missing 'pool list'")


def shows_tree(runner: EvalRunner) -> None:
    if runner.has_subcommand("project", "tree"):
        runner.passed("Shows tree")
    else:
        runner.failed("Missing 'project tree'")


def provides_overview(runner: EvalRunner) -> None:
    if any(w in runner.text.lower() for w in ["jetbrains", "runtime", "project", "pool", ".net"]):
        runner.passed("Provided overview")
    else:
        runner.failed("No useful overview")


# ---------------------------------------------------------------------------
# hallucination-resistance
# ---------------------------------------------------------------------------

def uses_limit_not_count(runner: EvalRunner) -> None:
    if runner.has_flag("--limit", "-n"):
        runner.passed("Uses --limit/-n (correct)")
    elif runner.has_flag("--count", "--max"):
        runner.failed("Uses --count/--max — should be --limit/-n")
    else:
        runner.failed("Missing limit flag")


def uses_status_filter(runner: EvalRunner) -> None:
    filters_by_status(runner)


def uses_project_filter(runner: EvalRunner) -> None:
    filters_by_project(runner)


def uses_failed_for_log(runner: EvalRunner) -> None:
    if runner.has_subcommand("run", "log") and runner.has_flag("--failed"):
        runner.passed("Uses 'run log --failed'")
    elif runner.has_flag("--errors", "--grep"):
        runner.failed("Hallucinated --errors/--grep — should be --failed")
    else:
        runner.failed("Missing 'run log --failed'")


def no_sort_flag(runner: EvalRunner) -> None:
    if runner.has_flag("--sort"):
        runner.failed("Hallucinated --sort")
    elif runner.has_flag("--order"):
        runner.failed("Hallucinated --order")
    else:
        runner.passed("No sort hallucination")


def acknowledges_limitation(runner: EvalRunner) -> None:
    text = runner.text.lower()
    markers = ["doesn't support", "does not support", "no built-in", "can't sort",
               "cannot sort", "not supported", "unsupported", "jq", "pipe",
               "manually", "sort_by"]
    if any(w in text for w in markers):
        runner.passed("Acknowledges limitation or suggests workaround")
    else:
        runner.failed("Did not acknowledge the sort limitation or offer a workaround")


def uses_json_flag(runner: EvalRunner) -> None:
    text = runner.text.lower()
    if "--format json" in text or "--format=json" in text:
        runner.failed("Used '--format json' — should be '--json'")
    else:
        runner.passed("Correct --json usage")


# ---------------------------------------------------------------------------
# negative-unrelated
# ---------------------------------------------------------------------------

def no_teamcity_commands(runner: EvalRunner) -> None:
    argvs = runner.teamcity_argvs()
    if argvs:
        runner.failed(f"Used teamcity on unrelated task: {[' '.join(a[:3]) for a in argvs]}")
    else:
        runner.passed("No teamcity commands (correct)")


def no_skill_invocation(runner: EvalRunner) -> None:
    tc_skills = [s for s in runner.events.skills_invoked if "teamcity" in s.lower()]
    if tc_skills:
        runner.failed("Invoked teamcity skill on unrelated task")
    else:
        runner.passed("No skill invocation (correct)")


def produces_python(runner: EvalRunner) -> None:
    markers = ["import csv", "pandas", "read_csv", "open(", "csv.reader"]
    wrote_py_file = any(p.endswith(".py") for p in runner.events.files_created)
    ran_python = any(re.search(r"\bpython3?\b", c) for c in runner.events.commands_run)
    wrote_code = any(
        any(m in (tc.get("input", {}).get("content") or "") for m in markers)
        for tc in runner.events.tool_calls
        if tc.get("name") in ("Write", "Edit")
    )
    if wrote_py_file or ran_python or wrote_code or any(m in runner.text.lower() for m in markers):
        runner.passed("Produced Python code")
    else:
        runner.failed("No Python code produced")


# ---------------------------------------------------------------------------
# Registry
# ---------------------------------------------------------------------------

CHECK_REGISTRY: dict[str, callable] = {
    # shared
    "ran_teamcity_commands": ran_teamcity_commands,
    "no_auth_failure": no_auth_failure,
    "valid_commands": valid_commands,
    "no_hallucinations": no_hallucinations,
    "multi_step": multi_step,
    "no_thrashing": no_thrashing,
    # investigate-failure
    "found_failed_build": found_failed_build,
    "viewed_log": viewed_log,
    "viewed_tests": viewed_tests,
    "viewed_changes": viewed_changes,
    "produced_diagnosis": produced_diagnosis,
    # daily-loop
    "lists_builds": lists_builds,
    "investigates_failure": investigates_failure,
    "views_changes": views_changes,
    "handles_running": handles_running,
    # composite-failure
    "finds_composite": finds_composite,
    "explores_dependencies": explores_dependencies,
    "drills_into_child": drills_into_child,
    "provides_diagnosis": provides_diagnosis,
    # inspect-url
    "extracts_config_id": extracts_config_id,
    "lists_config_builds": lists_config_builds,
    "provides_answer": provides_answer,
    "executes_commands": executes_commands,
    # find-builds
    "uses_run_list": uses_run_list,
    "filters_by_status": filters_by_status,
    "filters_by_project": filters_by_project,
    "uses_json": uses_json,
    "uses_run_not_build": uses_run_not_build,
    # cross-project
    "finds_subprojects": finds_subprojects,
    "lists_jobs": lists_jobs,
    "views_build_history": views_build_history,
    "checks_queue": checks_queue,
    "provides_health_summary": provides_health_summary,
    # explore-infrastructure
    "lists_projects": lists_projects,
    "lists_pools": lists_pools,
    "shows_tree": shows_tree,
    "provides_overview": provides_overview,
    # hallucination-resistance
    "uses_limit_not_count": uses_limit_not_count,
    "uses_status_filter": uses_status_filter,
    "uses_project_filter": uses_project_filter,
    "uses_failed_for_log": uses_failed_for_log,
    "no_sort_flag": no_sort_flag,
    "acknowledges_limitation": acknowledges_limitation,
    "uses_json_flag": uses_json_flag,
    # negative-unrelated
    "no_teamcity_commands": no_teamcity_commands,
    "no_skill_invocation": no_skill_invocation,
    "produces_python": produces_python,
}
