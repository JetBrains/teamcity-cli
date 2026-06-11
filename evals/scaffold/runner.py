"""EvalRunner — validation harness for eval checks.

Each check function receives an EvalRunner and calls runner.passed() / runner.failed().
All recorded results are deterministic checks; LLM grades are tracked separately by
the test runner so they can never blend into the gated pass rate.
"""

from __future__ import annotations

import re
import shlex
import traceback
from dataclasses import dataclass
from pathlib import PurePosixPath
from typing import Callable

from scaffold.events import ClaudeEvents

_ENV_ASSIGN = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")
_FLAG = re.compile(r"^-{1,2}[A-Za-z]")
_SEGMENT_SPLIT = re.compile(r"[|;&]+")


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
        """Commands actually executed via Bash tool calls. Text mentions don't count."""
        return self.events.commands_run

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

    # --- Command parsing -------------------------------------------------
    #
    # Executed Bash commands are tokenized (not substring-matched) so that a
    # project ID containing "build" or a URL never trips a command check.

    def teamcity_argvs(self) -> list[list[str]]:
        """Tokenized `teamcity ...` invocations from executed commands.

        Splits compound shell commands on |, ;, & and matches the binary by
        basename, so pipes and `bin/teamcity` are handled.
        """
        argvs: list[list[str]] = []
        for cmd in self.commands:
            for segment in _SEGMENT_SPLIT.split(cmd):
                try:
                    tokens = shlex.split(segment, posix=True)
                except ValueError:
                    tokens = segment.split()
                while tokens and _ENV_ASSIGN.match(tokens[0]):
                    tokens.pop(0)
                if tokens and PurePosixPath(tokens[0]).name == "teamcity":
                    argvs.append(tokens)
        return argvs

    @staticmethod
    def subcommand_tokens(argv: list[str]) -> list[str]:
        """Non-flag tokens after the binary, up to the first redirection."""
        path: list[str] = []
        for tok in argv[1:]:
            if tok.startswith("<") or tok.startswith(">") or tok.startswith("2>"):
                break
            if not _FLAG.match(tok):
                path.append(tok)
        return path

    @staticmethod
    def flag_tokens(argv: list[str]) -> list[str]:
        return [t for t in argv[1:] if _FLAG.match(t)]

    def has_subcommand(self, *path: str) -> bool:
        """True if any executed teamcity command starts with these subcommand tokens."""
        for argv in self.teamcity_argvs():
            tokens = self.subcommand_tokens(argv)
            if len(tokens) >= len(path) and all(tokens[i] == p for i, p in enumerate(path)):
                return True
        return False

    def has_flag(self, *flags: str) -> bool:
        """True if any executed teamcity command carries one of these exact flags."""
        for argv in self.teamcity_argvs():
            for tok in self.flag_tokens(argv):
                name = tok.split("=", 1)[0]
                if name in flags:
                    return True
        return False

    # --- Text matching ----------------------------------------------------

    def has_text(self, *fragments: str) -> bool:
        """Check if response text contains ALL fragments."""
        return all(f.lower() in self.text.lower() for f in fragments)

    def has_no_text(self, *fragments: str) -> bool:
        """Check that NONE of the fragments appear in the response text."""
        text_lower = self.text.lower()
        return not any(f.lower() in text_lower for f in fragments)

    def run(self, checks: list[Callable[[EvalRunner], None]]) -> list[CheckResult]:
        for check_fn in checks:
            self._current_check = check_fn.__name__
            try:
                check_fn(self)
            except Exception as e:
                self.failed(f"Exception: {e}\n{traceback.format_exc()}")
        self._current_check = ""
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
