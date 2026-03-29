"""TestRunner — validation harness for eval checks.

Each check function receives a TestRunner and calls runner.passed() / runner.failed().
"""

from __future__ import annotations

import json
import sys
import traceback
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable

from scaffold.events import ClaudeEvents


@dataclass
class CheckResult:
    name: str
    passed: bool
    message: str


class EvalRunner:
    """Runs validation checks against Claude's output."""

    def __init__(self, events: ClaudeEvents, task_name: str = ""):
        self.events = events
        self.task_name = task_name
        self._results: list[CheckResult] = []
        self._current_check: str = ""

    @property
    def text(self) -> str:
        return self.events.full_text

    @property
    def commands(self) -> list[str]:
        """All teamcity commands — both mentioned in text AND executed via Bash."""
        seen = set()
        result = []
        for cmd in self.events.commands_mentioned + self.events.commands_run:
            if cmd not in seen:
                seen.add(cmd)
                result.append(cmd)
        return result

    @property
    def tool_calls(self) -> list[dict]:
        return self.events.tool_calls

    @property
    def skills_invoked(self) -> list[str]:
        return self.events.skills_invoked

    def passed(self, msg: str) -> None:
        self._results.append(CheckResult(self._current_check, True, msg))

    def failed(self, msg: str) -> None:
        self._results.append(CheckResult(self._current_check, False, msg))

    def has_command(self, *fragments: str) -> bool:
        """Check if any mentioned command contains ALL fragments."""
        for cmd in self.commands:
            cmd_lower = cmd.lower()
            if all(f.lower() in cmd_lower for f in fragments):
                return True
        return False

    def has_text(self, *fragments: str) -> bool:
        """Check if response text OR executed commands contain ALL fragments."""
        combined = (self.text + "\n" + "\n".join(self.events.commands_run)).lower()
        return all(f.lower() in combined for f in fragments)

    def has_no_text(self, *fragments: str) -> bool:
        """Check that NONE of the fragments appear in the response text."""
        text_lower = self.text.lower()
        return not any(f.lower() in text_lower for f in fragments)

    def run(self, checks: list[Callable[[TestRunner], None]]) -> list[CheckResult]:
        for check_fn in checks:
            self._current_check = check_fn.__name__
            try:
                check_fn(self)
            except Exception as e:
                self.failed(f"Exception: {e}\n{traceback.format_exc()}")
        return self._results

    @property
    def passed_count(self) -> int:
        return sum(1 for r in self._results if r.passed)

    @property
    def failed_count(self) -> int:
        return sum(1 for r in self._results if not r.passed)

    @property
    def total_count(self) -> int:
        return len(self._results)

    @property
    def pass_rate(self) -> float:
        if not self._results:
            return 0.0
        return self.passed_count / self.total_count

    def summary(self) -> dict:
        return {
            "task": self.task_name,
            "passed": self.passed_count,
            "failed": self.failed_count,
            "total": self.total_count,
            "pass_rate": round(self.pass_rate, 3),
            "results": [
                {"check": r.name, "passed": r.passed, "message": r.message}
                for r in self._results
            ],
        }

    def print_summary(self) -> None:
        s = self.summary()
        status = "PASS" if s["failed"] == 0 else "FAIL"
        print(f"\n{'='*60}")
        print(f"  {s['task']}  [{status}]  {s['passed']}/{s['total']} checks passed")
        print(f"{'='*60}")
        for r in s["results"]:
            icon = "+" if r["passed"] else "x"
            print(f"  [{icon}] {r['check']}: {r['message']}")
        print()
