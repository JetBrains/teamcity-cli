"""Pytest configuration — CLI options, fixtures, parametrization."""

from __future__ import annotations

import os
import uuid
from dataclasses import dataclass
from pathlib import Path

import pytest

from scaffold.tasks import TaskConfig, list_tasks, load_task

SKILLS_DIR = Path(__file__).resolve().parent.parent / "skills"


@dataclass
class TreatmentConfig:
    name: str
    skill_dir: Path | None = None


TREATMENTS = {
    "CONTROL": TreatmentConfig(name="CONTROL", skill_dir=None),
    "CURRENT": TreatmentConfig(name="CURRENT", skill_dir=SKILLS_DIR / "teamcity-cli"),
}


# ---------------------------------------------------------------------------
# CLI options
# ---------------------------------------------------------------------------

def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption("--task", default=None, help="Task name(s), comma-separated")
    parser.addoption("--treatment", default=None, help="Treatment name(s), comma-separated")
    parser.addoption("--runs", default=1, type=int, help="Repetitions per combination")
    parser.addoption("--experiment", default=None, help="Experiment name for LangSmith tagging")


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

@pytest.fixture(scope="session", autouse=True)
def verify_env() -> None:
    """Fail fast if required env vars are missing or TeamCity auth is broken."""
    import subprocess

    missing = [k for k in ("TEAMCITY_URL", "TEAMCITY_TOKEN", "ANTHROPIC_API_KEY")
               if not os.environ.get(k)]
    assert not missing, f"Missing required env vars: {', '.join(missing)}"

    import shutil
    assert shutil.which("teamcity"), "teamcity CLI not found on PATH"

    result = subprocess.run(
        ["teamcity", "auth", "status", "--no-input"],
        capture_output=True, text=True, timeout=15,
        env={**os.environ, "NO_COLOR": "1"},
    )
    assert result.returncode == 0, (
        f"TeamCity auth failed (is TEAMCITY_TOKEN valid for {os.environ['TEAMCITY_URL']}?):\n"
        f"{result.stdout}{result.stderr}"
    )


@pytest.fixture(scope="session", autouse=True)
def verify_skill() -> None:
    """Verify the skill exists and print its version."""
    skill_md = SKILLS_DIR / "teamcity-cli" / "SKILL.md"
    assert skill_md.exists(), f"Skill not found at {skill_md}"

    version = "unknown"
    for line in skill_md.read_text().splitlines():
        if line.startswith("version:"):
            version = line.split(":", 1)[1].strip().strip('"')
            break

    print(f"\n  Skill: {skill_md.parent}")
    print(f"  Version: {version}\n")


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
