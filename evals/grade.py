"""Outcome grading: compare the file the agent left behind with the task's reference.

A task's tests/expected.json names the file and the reference:

    {"file": "answer.json", "answer": {...}, "ordered": ["runs"], "prefix": ["commit"]}

Each top-level key of `answer` is one graded field. Strings compare case-insensitively, numbers
by value (so "12" equals 12), lists as multisets unless the field is listed in `ordered`, and a
`prefix` field accepts any 7+ character prefix of the reference (commit hashes). A script task
uses {"file": ..., "run": [...], "files": {...}, "stdout": "..."} instead: the file is executed
next to the given fixture files and its output must contain `stdout`.
"""

from __future__ import annotations

import json
import re
import subprocess
import tomllib
from pathlib import Path


_NUMBER = re.compile(r"-?\d+(\.\d+)?")


def _norm(value):
    if isinstance(value, bool) or value is None:
        return value
    if isinstance(value, (int, float)):
        return float(value)
    if isinstance(value, str):
        text = value.strip().casefold()
        return float(text) if _NUMBER.fullmatch(text) else text
    if isinstance(value, list):
        return [_norm(v) for v in value]
    if isinstance(value, dict):
        return {str(k).strip().casefold(): _norm(v) for k, v in value.items()}
    return value


def _same(expected, got, ordered: bool) -> bool:
    """Deep equality on normalized values; lists ignore order unless `ordered`."""
    if isinstance(expected, dict):
        return isinstance(got, dict) and expected.keys() == got.keys() and all(
            _same(v, got[k], ordered) for k, v in expected.items())
    if isinstance(expected, list):
        if not isinstance(got, list) or len(expected) != len(got):
            return False
        if ordered:
            return all(_same(e, g, ordered) for e, g in zip(expected, got))
        left = list(got)
        for e in expected:
            match = next((i for i, g in enumerate(left) if _same(e, g, ordered)), None)
            if match is None:
                return False
            left.pop(match)
        return True
    return type(expected) is type(got) and expected == got


def _result(check: str, passed: bool, message: str) -> dict:
    return {"check": check, "passed": passed, "message": message}


def _load(path: Path):
    if path.suffix == ".toml":
        return tomllib.loads(path.read_text())
    return json.loads(path.read_text())


def _grade_script(spec: dict, workdir: Path) -> list[dict]:
    for name, content in spec.get("files", {}).items():
        (workdir / name).write_text(content)
    try:
        out = subprocess.run(spec["run"], cwd=workdir, capture_output=True, text=True, timeout=60)
    except (OSError, subprocess.TimeoutExpired) as e:
        return [_result("output", False, f"could not run {spec['file']}: {e}")]
    ok = out.returncode == 0 and spec["stdout"] in out.stdout
    return [_result("output", ok, "printed the expected value" if ok else
                    f"exit {out.returncode}, output {out.stdout.strip()[:200]!r} lacks {spec['stdout']!r}")]


def grade(spec: dict, workdir: Path) -> list[dict]:
    """One result per graded field; all fail when the file is missing or unreadable."""
    path = workdir / spec["file"]
    if "run" in spec:
        return _grade_script(spec, workdir) if path.is_file() else [_result("output", False, f"{spec['file']} not found")]
    expected = spec["answer"]
    try:
        answer = _load(path)
        if not isinstance(answer, dict):
            raise ValueError("top level is not an object")
    except (OSError, ValueError) as e:
        reason = f"{spec['file']} not found" if not path.is_file() else f"{spec['file']} is not valid: {e}"
        return [_result(field, False, reason) for field in expected]
    raw = {str(k).strip().casefold(): v for k, v in answer.items()}
    results = []
    for field, want in expected.items():
        got = raw.get(field.casefold())
        if field in spec.get("prefix", []):
            ok = isinstance(got, str) and len(got.strip()) >= 7 and want.casefold().startswith(got.strip().casefold())
        else:
            ok = _same(_norm(want), _norm(got), field in spec.get("ordered", []))
        results.append(_result(field, ok, "matches the reference" if ok else f"got {json.dumps(got)[:300]}"))
    return results


def score(results: list[dict]) -> float:
    return round(sum(r["passed"] for r in results) / len(results), 3) if results else 0.0
