"""Task loader — reads tasks.json."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path

TASKS_FILE = Path(__file__).resolve().parent.parent / "tasks.json"


@dataclass
class TaskConfig:
    name: str
    instruction: str
    checks: list[str] = field(default_factory=list)
    llm_grade: bool = False
    default_treatments: list[str] = field(default_factory=lambda: ["CONTROL", "CURRENT"])


def _load_all() -> list[dict]:
    with open(TASKS_FILE) as f:
        return json.load(f)


def load_task(task_name: str) -> TaskConfig:
    for t in _load_all():
        if t["id"] == task_name:
            return TaskConfig(
                name=t["id"],
                instruction=t["prompt"],
                checks=t.get("checks", []),
                llm_grade=t.get("llm_grade", False),
                default_treatments=t.get("treatments", ["CONTROL", "CURRENT"]),
            )
    raise KeyError(f"Task '{task_name}' not found in {TASKS_FILE}")


def list_tasks() -> list[str]:
    return [t["id"] for t in _load_all()]
