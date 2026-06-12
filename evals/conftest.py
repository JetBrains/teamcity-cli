"""Pytest configuration — CLI options, fixtures, parametrization."""

from __future__ import annotations

import hashlib
import os
import shutil
import subprocess
import uuid
from dataclasses import dataclass
from pathlib import Path

import pytest

from scaffold import sentry_log
from scaffold.claude import CLAUDE_BIN, DEFAULT_MODEL
from scaffold.tasks import TASKS_FILE, TaskConfig, list_tasks, load_task

EVALS_DIR = Path(__file__).resolve().parent
REPO_ROOT = EVALS_DIR.parent
SKILLS_DIR = REPO_ROOT / "skills"
SKILL_MD = SKILLS_DIR / "teamcity-cli" / "SKILL.md"
SCHEMA_PATH = EVALS_DIR / "cli_schema.json"


@dataclass
class TreatmentConfig:
    name: str
    skill_dir: Path | None = None


TREATMENTS = {
    "CONTROL": TreatmentConfig(name="CONTROL", skill_dir=None),
    "CURRENT": TreatmentConfig(name="CURRENT", skill_dir=SKILLS_DIR / "teamcity-cli"),
}

# Rough historical durations, longest first, so xdist doesn't park the slow
# tasks on a near-empty tail.
TASK_DURATION_RANK = [
    "composite-failure", "cross-project", "daily-loop", "investigate-failure",
    "inspect-url", "find-builds", "hallucination-resistance",
    "explore-infrastructure", "negative-unrelated",
]


# ---------------------------------------------------------------------------
# CLI options + session setup
# ---------------------------------------------------------------------------

def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption("--task", default=None, help="Task name(s), comma-separated")
    parser.addoption("--treatment", default=None, help="Treatment name(s), comma-separated")
    parser.addoption("--runs", default=1, type=int, help="Repetitions per combination")
    parser.addoption("--experiment", default=None, help="Experiment ID tag (defaults to branch name)")


def pytest_configure(config: pytest.Config) -> None:
    """Regenerate cli_schema.json from the cobra tree (controller only)."""
    if hasattr(config, "workerinput"):
        return  # xdist worker — the controller already generated it
    generator = REPO_ROOT / "scripts" / "generate-cli-schema.go"
    if shutil.which("go") and generator.exists():
        out = subprocess.run(
            ["go", "run", str(generator)],
            cwd=REPO_ROOT, capture_output=True, text=True, timeout=180,
        )
        if out.returncode == 0:
            SCHEMA_PATH.write_text(out.stdout)
            return
        if not SCHEMA_PATH.exists():
            raise pytest.UsageError(f"cli_schema.json generation failed:\n{out.stderr}")
    elif not SCHEMA_PATH.exists():
        raise pytest.UsageError(
            "cli_schema.json missing and `go` not on PATH — run "
            "`go run scripts/generate-cli-schema.go > evals/cli_schema.json`"
        )


# ---------------------------------------------------------------------------
# Dynamic test parametrization
# ---------------------------------------------------------------------------

def pytest_generate_tests(metafunc: pytest.Metafunc) -> None:
    if "task_config" not in metafunc.fixturenames:
        return

    task_filter = metafunc.config.getoption("--task")
    treatment_filter = metafunc.config.getoption("--treatment")
    count = metafunc.config.getoption("--runs")

    task_names = (
        [t.strip() for t in task_filter.split(",")]
        if task_filter else list_tasks()
    )
    task_names.sort(
        key=lambda t: TASK_DURATION_RANK.index(t) if t in TASK_DURATION_RANK else 99
    )

    treatment_names = (
        [t.strip() for t in treatment_filter.split(",")]
        if treatment_filter else None
    )

    combos = []
    for tn in task_names:
        tc = load_task(tn)
        names = treatment_names or tc.default_treatments
        for tr_name in names:
            tr = TREATMENTS[tr_name]
            for i in range(count):
                label = f"{tn}--{tr_name}"
                if count > 1:
                    label += f"--run{i+1}"
                combos.append(pytest.param(tc, tr, id=label))

    metafunc.parametrize("task_config,treatment_config", combos)


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture(scope="session")
def verify_env() -> None:
    """Fail fast if required env vars are missing or TeamCity auth is broken.

    Requested by the live eval tests only — unit tests run without a server.
    """
    missing = [k for k in ("TEAMCITY_URL", "TEAMCITY_TOKEN")
               if not os.environ.get(k)]
    assert not missing, f"Missing required env vars: {', '.join(missing)}"

    assert shutil.which("teamcity"), "teamcity CLI not found on PATH"
    assert shutil.which("claude"), "claude CLI not found on PATH"

    result = subprocess.run(
        ["teamcity", "auth", "status", "--no-input"],
        capture_output=True, text=True, timeout=15,
        env={**os.environ, "NO_COLOR": "1"},
    )
    assert result.returncode == 0, (
        f"TeamCity auth failed (is TEAMCITY_TOKEN valid for {os.environ['TEAMCITY_URL']}?):\n"
        f"{result.stdout}{result.stderr}"
    )


def _skill_version() -> str:
    for line in SKILL_MD.read_text().splitlines():
        if line.startswith("version:"):
            return line.split(":", 1)[1].strip().strip('"')
    return "unknown"


@pytest.fixture(scope="session")
def verify_skill() -> None:
    """Verify the skill exists and print its version."""
    assert SKILL_MD.exists(), f"Skill not found at {SKILL_MD}"
    print(f"\n  Skill: {SKILL_MD.parent}")
    print(f"  Version: {_skill_version()}\n")


def _cmd_output(cmd: list[str]) -> str:
    try:
        out = subprocess.run(cmd, capture_output=True, text=True, timeout=15)
        return out.stdout.strip().splitlines()[0] if out.stdout.strip() else ""
    except Exception:
        return ""


@pytest.fixture(scope="session")
def provenance() -> dict:
    """Version-stamp every result so runs are comparable across time."""
    git_sha = (
        os.environ.get("BUILD_VCS_NUMBER")
        or _cmd_output(["git", "-C", str(REPO_ROOT), "rev-parse", "HEAD"])
    )
    return {
        "git_sha": git_sha[:12],
        "cli_version": _cmd_output(["teamcity", "--version"]),
        "claude_version": _cmd_output([CLAUDE_BIN, "--version"]),
        "model": os.environ.get("BENCH_CC_MODEL") or DEFAULT_MODEL,
        "skill_version": _skill_version(),
        "task_set": hashlib.sha256(TASKS_FILE.read_bytes()).hexdigest()[:12],
    }


@pytest.fixture(scope="session", autouse=True)
def sentry_session():
    """Initialize Sentry once per pytest session; flush at teardown."""
    active = sentry_log.init_if_configured()
    if active:
        print("  Sentry: enabled\n")
    yield
    if active:
        sentry_log.flush(timeout=10.0)


@pytest.fixture(scope="session")
def experiment_id(request, tmp_path_factory) -> str:
    # With xdist, each worker has its own session. Use a shared file so all
    # workers report the same experiment_id.
    name = request.config.getoption("--experiment")
    if not name:
        name = os.environ.get("BRANCH_NAME", "").replace("/", "_")
    if not name:
        # Generate once, share via root tmp dir (xdist shares this)
        root = tmp_path_factory.getbasetemp().parent
        id_file = root / "experiment_id"
        if id_file.exists():
            name = id_file.read_text().strip()
        else:
            name = f"eval-{uuid.uuid4().hex[:8]}"
            id_file.write_text(name)
    return name


@pytest.fixture
def run_id() -> str:
    return uuid.uuid4().hex[:12]
