"""CHECK_REGISTRY — all validation functions in one place.

Each check takes an EvalRunner and calls runner.passed() or runner.failed().
Checks are referenced by ID in tasks.json.
"""

from __future__ import annotations

import re

from scaffold.runner import EvalRunner

# ---------------------------------------------------------------------------
# Shared: valid commands and hallucination detection
# ---------------------------------------------------------------------------

VALID_SUBCOMMANDS = {
    "auth": ["login", "logout", "status"],
    "run": [
        "list", "view", "start", "cancel", "restart", "watch", "log",
        "tests", "changes", "artifacts", "download", "pin", "unpin",
        "tag", "untag", "comment",
    ],
    "job": ["list", "view", "tree", "pause", "resume", "param"],
    "project": ["list", "view", "tree", "param", "token", "settings"],
    "queue": ["list", "remove", "top", "approve"],
    "agent": [
        "list", "view", "authorize", "deauthorize", "enable", "disable",
        "move", "jobs", "exec", "term", "reboot",
    ],
    "pool": ["list", "view", "link", "unlink"],
    "api": [],
    "alias": ["set", "list", "delete"],
    "skill": ["install", "update", "remove"],
}

KNOWN_HALLUCINATIONS = [
    "--format", "--output-format", "--type", "--config",
    "--build-type", "--configuration", "--build-config",
    "--project-id", "--build-id", "--agent-id", "--pool-id",
    "--count", "--max", "--tail", "--follow",
]

VALIDATE_WITH_DOT_PATH_RE = re.compile(
    r"teamcity\s+project\s+settings\s+validate\s+\.(?:\s|$)",
    re.IGNORECASE,
)
MAVEN_COMMAND_RE = re.compile(r"(?:^|\s)(?:\./)?mvnw?(?:\s|$)", re.IGNORECASE)


def ran_teamcity_commands(runner: EvalRunner) -> None:
    """Claude must have actually executed teamcity commands, not just talked about them."""
    tc_cmds = [c for c in runner.events.commands_run if "teamcity" in c]
    if tc_cmds:
        runner.passed(f"Executed {len(tc_cmds)} teamcity command(s)")
    else:
        runner.failed("Did not execute any teamcity commands")


def no_auth_failure(runner: EvalRunner) -> None:
    """Fail if Claude got stuck on authentication instead of completing the task."""
    text = runner.text.lower()
    auth_phrases = ["authentication failed", "not authenticated", "unauthorized",
                    "401", "please provide", "need a token", "need credentials",
                    "i don't have access", "i cannot access", "authentication issue",
                    "could not authenticate"]
    if any(p in text for p in auth_phrases) and not any("teamcity" in c for c in runner.events.commands_run):
        runner.failed("Got stuck on authentication without completing the task")
    else:
        runner.passed("No auth failure blocking task completion")


def valid_commands(runner: EvalRunner) -> None:
    invalid = []
    for cmd in runner.commands:
        parts = cmd.split()
        if len(parts) < 2 or parts[0] != "teamcity":
            continue
        group = parts[1]
        if group.startswith("-"):
            continue
        if group not in VALID_SUBCOMMANDS:
            invalid.append(f"Unknown '{group}' in: {cmd}")
            continue
        if len(parts) >= 3 and VALID_SUBCOMMANDS[group]:
            sub = parts[2]
            if not sub.startswith("-") and sub not in VALID_SUBCOMMANDS[group]:
                invalid.append(f"Unknown '{group} {sub}' in: {cmd}")
    if invalid:
        runner.failed(f"Invalid commands: {'; '.join(invalid)}")
    else:
        runner.passed("All commands are valid")


def no_hallucinations(runner: EvalRunner) -> None:
    violations = []
    for cmd in runner.commands:
        for flag in KNOWN_HALLUCINATIONS:
            if flag in cmd:
                if flag == "--dry-run" and "run start" in cmd:
                    continue
                if flag == "-f " and ("cancel" in cmd or "remove" in cmd):
                    continue
                violations.append(f"'{flag}' in: {cmd}")
    if violations:
        runner.failed(f"Hallucinated flags: {'; '.join(violations)}")
    else:
        runner.passed("No hallucinated flags")


# ---------------------------------------------------------------------------
# investigate-failure
# ---------------------------------------------------------------------------

def found_failed_build(runner: EvalRunner) -> None:
    if runner.has_command("run", "list") and runner.has_text("failure"):
        runner.passed("Found failed builds")
    elif runner.has_command("run", "view"):
        runner.passed("Inspected a build")
    else:
        runner.failed("Did not search for builds")


def viewed_log(runner: EvalRunner) -> None:
    if runner.has_command("run", "log"):
        runner.passed("Checked build log")
    else:
        runner.failed("Missing 'run log'")


def viewed_tests(runner: EvalRunner) -> None:
    if runner.has_command("run", "tests"):
        runner.passed("Checked test results")
    else:
        runner.failed("Missing 'run tests'")


def viewed_changes(runner: EvalRunner) -> None:
    if runner.has_command("run", "changes"):
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

def finds_user_builds(runner: EvalRunner) -> None:
    if runner.has_command("--user") or runner.has_command("-u"):
        runner.passed("Filters by user")
    elif runner.has_text("viktor") or runner.has_text("tiulpin"):
        runner.passed("References the user")
    else:
        runner.failed("Did not filter by user")


def lists_builds(runner: EvalRunner) -> None:
    if runner.has_command("run", "list"):
        runner.passed("Lists builds")
    else:
        runner.failed("Missing 'run list'")


def investigates_failure(runner: EvalRunner) -> None:
    if runner.has_command("run", "log") and runner.has_command("run", "tests"):
        runner.passed("Checks log and tests")
    elif runner.has_command("run", "log") or runner.has_command("run", "tests"):
        runner.passed("Checks log or tests")
    else:
        runner.failed("Did not investigate failure details")


def views_changes(runner: EvalRunner) -> None:
    viewed_changes(runner)


def handles_running(runner: EvalRunner) -> None:
    if runner.has_command("run", "watch"):
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


# ---------------------------------------------------------------------------
# composite-failure
# ---------------------------------------------------------------------------

def finds_composite(runner: EvalRunner) -> None:
    if runner.has_command("run", "list") or runner.has_command("run", "view"):
        runner.passed("Finds builds")
    else:
        runner.failed("Did not search for builds")


def explores_dependencies(runner: EvalRunner) -> None:
    if runner.has_command("job", "tree"):
        runner.passed("Uses 'job tree' for dependencies")
    elif runner.has_command("run", "list") and runner.has_command("--status"):
        runner.passed("Filters by failure status")
    elif runner.has_command("api"):
        runner.passed("Uses API for dependencies")
    else:
        runner.failed("Did not explore dependency chain")


def drills_into_child(runner: EvalRunner) -> None:
    if runner.has_command("run", "log") or runner.has_command("run", "tests"):
        runner.passed("Inspects child build details")
    else:
        runner.failed("Did not inspect child build")


def provides_diagnosis(runner: EvalRunner) -> None:
    produced_diagnosis(runner)


# ---------------------------------------------------------------------------
# inspect-url
# ---------------------------------------------------------------------------

def extracts_config_id(runner: EvalRunner) -> None:
    if runner.has_text("ijplatform_master_CIDR_CLion_CLionTrunkHeavyTests_TestsUbuntu2404x86_64"):
        runner.passed("Extracted config ID from URL")
    else:
        runner.failed("Did not extract config ID from URL")


def lists_config_builds(runner: EvalRunner) -> None:
    if runner.has_command("run", "list"):
        runner.passed("Listed builds for the configuration")
    elif runner.has_command("run", "view"):
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
    if runner.has_command("--status"):
        runner.passed("Filters by status")
    else:
        runner.failed("Missing --status")


def filters_by_project(runner: EvalRunner) -> None:
    if runner.has_command("--project") or runner.has_command("-p"):
        runner.passed("Filters by project")
    else:
        runner.failed("Missing --project")


def uses_since(runner: EvalRunner) -> None:
    if runner.has_command("--since") or runner.has_text("24h"):
        runner.passed("Uses --since")
    else:
        runner.failed("Missing --since")


def uses_json(runner: EvalRunner) -> None:
    if runner.has_command("--json"):
        runner.passed("Uses --json")
    else:
        runner.failed("Missing --json")


def uses_run_not_build(runner: EvalRunner) -> None:
    if runner.has_command("teamcity", "build"):
        runner.failed("Used 'teamcity build' — should be 'teamcity run'")
    else:
        runner.passed("Correct: uses 'run' not 'build'")


# ---------------------------------------------------------------------------
# cross-project
# ---------------------------------------------------------------------------

def finds_subprojects(runner: EvalRunner) -> None:
    has_list = runner.has_command("project", "list") or runner.has_command("project", "tree")
    has_jcef = runner.has_text("JCEF") or runner.has_text("jcef") or runner.has_text("JBR_JCEF")
    if has_list and has_jcef:
        runner.passed("Found JCEF subprojects")
    elif has_list:
        runner.passed("Listed projects")
    else:
        runner.failed("Did not find subprojects")


def lists_jobs(runner: EvalRunner) -> None:
    if runner.has_command("job", "list"):
        runner.passed("Lists jobs")
    else:
        runner.failed("Missing 'job list'")


def views_build_history(runner: EvalRunner) -> None:
    lists_builds(runner)


def checks_queue(runner: EvalRunner) -> None:
    if runner.has_command("queue", "list"):
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
    if runner.has_command("project", "list"):
        runner.passed("Lists projects")
    else:
        runner.failed("Missing 'project list'")


def lists_pools(runner: EvalRunner) -> None:
    if runner.has_command("pool", "list"):
        runner.passed("Lists pools")
    else:
        runner.failed("Missing 'pool list'")


def shows_tree(runner: EvalRunner) -> None:
    if runner.has_command("project", "tree"):
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
    if runner.has_command("--limit") or runner.has_command("-n"):
        runner.passed("Uses --limit/-n (correct)")
    elif runner.has_command("--count") or runner.has_command("--max"):
        runner.failed("Uses --count/--max — should be --limit/-n")
    else:
        runner.failed("Missing limit flag")


def uses_status_filter(runner: EvalRunner) -> None:
    filters_by_status(runner)


def uses_project_filter(runner: EvalRunner) -> None:
    filters_by_project(runner)


def uses_failed_for_log(runner: EvalRunner) -> None:
    if runner.has_command("run", "log") and runner.has_command("--failed"):
        runner.passed("Uses 'run log --failed'")
    elif runner.has_command("--errors") or runner.has_command("--grep"):
        runner.failed("Hallucinated --errors/--grep — should be --failed")
    else:
        runner.failed("Missing 'run log --failed'")


def no_sort_flag(runner: EvalRunner) -> None:
    if runner.has_command("--sort"):
        runner.failed("Hallucinated --sort")
    elif runner.has_command("--order"):
        runner.failed("Hallucinated --order")
    else:
        runner.passed("No sort hallucination")


def acknowledges_limitation(runner: EvalRunner) -> None:
    text = runner.text.lower()
    if any(w in text for w in ["doesn't support", "no built-in", "can't sort", "cannot sort", "jq", "pipe", "manually"]):
        runner.passed("Acknowledges limitation or suggests workaround")
    else:
        runner.passed("Acceptable (no hallucination)")


def uses_json_flag(runner: EvalRunner) -> None:
    text = runner.text.lower()
    if "--format json" in text or "--format=json" in text:
        runner.failed("Used '--format json' — should be '--json'")
    else:
        runner.passed("Correct --json usage")


# ---------------------------------------------------------------------------
# kotlin-dsl-validate-workflow
# ---------------------------------------------------------------------------

def uses_project_settings_validate(runner: EvalRunner) -> None:
    if runner.has_command("teamcity", "project", "settings", "validate"):
        runner.passed("Uses 'teamcity project settings validate'")
    else:
        runner.failed("Missing 'teamcity project settings validate'")


def validates_with_explicit_dot_path(runner: EvalRunner) -> None:
    validate_cmds = [
        cmd for cmd in runner.events.commands_run
        if "teamcity project settings validate" in cmd.lower()
    ]
    if not validate_cmds:
        runner.failed("No validate command found")
        return

    has_dot_path = any(
        VALIDATE_WITH_DOT_PATH_RE.search(cmd)
        for cmd in validate_cmds
    )
    if has_dot_path:
        runner.passed("Uses explicit '.' path for root-layout DSL")
    else:
        runner.failed("Did not pass explicit '.' path to validate")


def avoids_raw_maven_for_dsl_validation(runner: EvalRunner) -> None:
    maven_cmds = [
        cmd for cmd in runner.events.commands_run
        if MAVEN_COMMAND_RE.search(cmd)
    ]
    if maven_cmds:
        runner.failed(f"Used raw Maven for DSL validation (found {len(maven_cmds)} command(s))")
    else:
        runner.passed("Avoids raw Maven validation commands")


# ---------------------------------------------------------------------------
# negative-unrelated
# ---------------------------------------------------------------------------

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
# negative-unrelated
# ---------------------------------------------------------------------------

def no_teamcity_commands(runner: EvalRunner) -> None:
    tc_cmds = [c for c in runner.events.commands_run if "teamcity" in c.lower()]
    if tc_cmds:
        runner.failed(f"Used teamcity on unrelated task: {tc_cmds}")
    else:
        runner.passed("No teamcity commands (correct)")


def no_skill_invocation(runner: EvalRunner) -> None:
    tc_skills = [s for s in runner.events.skills_invoked if "teamcity" in s.lower()]
    if tc_skills:
        runner.failed(f"Invoked teamcity skill on unrelated task")
    else:
        runner.passed("No skill invocation (correct)")


def produces_python(runner: EvalRunner) -> None:
    if any(w in runner.text.lower() for w in ["import csv", "pandas", "read_csv", "open(", "csv.reader"]):
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
    # kotlin-dsl-validate-workflow
    "uses_project_settings_validate": uses_project_settings_validate,
    "validates_with_explicit_dot_path": validates_with_explicit_dot_path,
    "avoids_raw_maven_for_dsl_validation": avoids_raw_maven_for_dsl_validation,
    # negative
    "no_thrashing": no_thrashing,
    # negative
    "no_teamcity_commands": no_teamcity_commands,
    "no_skill_invocation": no_skill_invocation,
    "produces_python": produces_python,
}
