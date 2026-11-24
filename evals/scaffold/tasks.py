"""Task loader — reads Harbor-format task directories under evals/tasks/.

Each task is `tasks/<id>/` with `instruction.md` (the prompt), `task.toml`
(Harbor settings; harness options under [metadata]), `tests/checks.toml`
(process and safety check IDs from CHECK_REGISTRY), `tests/expected.json`
(the reference answer), `solution/solve.sh` (the oracle) and optional
`environment/workspace/` files seeded into the agent's working directory.
"""

from __future__ import annotations

import tomllib
from dataclasses import dataclass, field
from pathlib import Path

TASKS_DIR = Path(__file__).resolve().parent.parent / "tasks"


@dataclass
class TaskConfig:
    name: str
    instruction: str
    checks: list[str] = field(default_factory=list)  # process checks, advisory
    safety: list[str] = field(default_factory=list)  # gates that zero the reward
    llm_grade: bool = False
    default_treatments: list[str] = field(default_factory=lambda: ["CONTROL", "CURRENT"])
    setup_files: dict[str, str] = field(default_factory=dict)


def _task_dirs() -> list[Path]:
    return sorted(p for p in TASKS_DIR.iterdir() if (p / "task.toml").is_file())


def load_task(task_name: str) -> TaskConfig:
    d = TASKS_DIR / task_name
    if not (d / "task.toml").is_file():
        raise KeyError(f"Task '{task_name}' not found in {TASKS_DIR}")
    meta = tomllib.loads((d / "task.toml").read_text()).get("metadata", {})
    checks = tomllib.loads((d / "tests" / "checks.toml").read_text())
    workspace = d / "environment" / "workspace"
    return TaskConfig(
        name=task_name,
        instruction=(d / "instruction.md").read_text().strip(),
        checks=checks.get("checks", []),
        safety=checks.get("safety", []),
        llm_grade=meta.get("llm_grade", False),
        default_treatments=meta.get("treatments", ["CONTROL", "CURRENT"]),
        setup_files={
            str(p.relative_to(workspace)): p.read_text()
            for p in sorted(workspace.rglob("*")) if p.is_file()
        } if workspace.is_dir() else {},
    )


def list_tasks() -> list[str]:
    return [p.name for p in _task_dirs()]


def task_set_files() -> list[Path]:
    """Every file that defines the task set, for the provenance hash."""
    return sorted(f for d in _task_dirs() for f in d.rglob("*") if f.is_file())
