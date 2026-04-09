#!/usr/bin/env python3
"""Compare eval experiments via LangSmith or local result directories.

Queries LangSmith for runs tagged with experiment_id (= branch name in CI).
Compares current branch against main automatically.

Regression gate: fails if the OVERALL average of CURRENT runs drops >10%.
Individual task swings are reported but don't gate — single-run variance is too high.

Usage:
    uv run scripts/compare.py                          # current branch vs main (LangSmith)
    uv run scripts/compare.py feature_x main           # explicit A vs B (LangSmith)
    uv run scripts/compare.py results/eval-X results/eval-Y  # local directories
"""

from __future__ import annotations

import json
import os
import sys
from collections import defaultdict
from pathlib import Path


def avg(values: list[float]) -> float:
    return sum(values) / len(values) if values else 0


def print_comparison(label_a: str, label_b: str, summary_a: dict, summary_b: dict, count_a: int, count_b: int) -> None:
    all_keys = sorted(set(list(summary_a.keys()) + list(summary_b.keys())))

    print(f"\n  {label_a} ({count_a} runs) → {label_b} ({count_b} runs)\n")
    print(f"  {'Task/Treatment':<45s}  {label_a:>13s}  {label_b:>13s}  {'Delta':>7s}")
    print(f"  {'-'*84}")

    current_deltas = []
    for key in all_keys:
        a = summary_a.get(key, {})
        b = summary_b.get(key, {})
        pa = a.get("pass_rate", 0)
        pb = b.get("pass_rate", 0)

        if a and b:
            delta = pb - pa
            is_current = "/CURRENT" in key
            marker = ""
            if is_current and delta < -0.15:
                marker = "  ↓"
            elif is_current and delta > 0.15:
                marker = "  ↑"
            print(f"  {key:<45s}  {pa:>12.0%}  {pb:>12.0%}   {delta:>+5.0%}{marker}")
            if is_current:
                current_deltas.append(delta)
        elif b:
            print(f"  {key:<45s}  {'  -':>13s}  {pb:>12.0%}    new")
        else:
            print(f"  {key:<45s}  {pa:>12.0%}  {'  -':>13s}   gone")

    print(f"  {'-'*84}")
    if current_deltas:
        overall = avg(current_deltas)
        print(f"  {'CURRENT AVERAGE':<45s}  {' ':>13s}  {' ':>12s}  {overall:>+5.0%}")
        print()

        if overall < -0.10:
            print(f"  REGRESSION: overall CURRENT average dropped {overall:+.0%} (threshold: -10%)")
            sys.exit(1)
        else:
            print(f"  No regression (overall CURRENT: {overall:+.0%})")
    else:
        print()
        print(f"  No CURRENT runs to compare.")


def load_local_dir(path: Path) -> tuple[dict, int]:
    results: dict[str, dict] = defaultdict(lambda: {"pass_rates": []})
    count = 0
    for f in sorted(path.glob("*.json")):
        if f.name.endswith(".raw.jsonl"):
            continue
        data = json.loads(f.read_text())
        task = data.get("task", "")
        if task:
            results[task]["pass_rates"].append(data.get("pass_rate", 0))
            count += 1
    for data in results.values():
        data["pass_rate"] = avg(data["pass_rates"])
        data["n"] = len(data["pass_rates"])
    return dict(results), count


def compare_local(path_a: str, path_b: str) -> None:
    dir_a = Path(path_a)
    dir_b = Path(path_b)
    if not dir_a.is_dir():
        print(f"Not a directory: {path_a}")
        sys.exit(1)
    if not dir_b.is_dir():
        print(f"Not a directory: {path_b}")
        sys.exit(1)

    summary_a, count_a = load_local_dir(dir_a)
    summary_b, count_b = load_local_dir(dir_b)
    print_comparison(dir_a.name, dir_b.name, summary_a, summary_b, count_a, count_b)


def compare_langsmith(arg_a: str | None, arg_b: str | None) -> None:
    from langsmith import Client

    client = Client()
    project = os.environ.get("LANGSMITH_PROJECT", "teamcity-cli")

    runs = list(client.list_runs(project_name=project, limit=100, is_root=True))
    by_exp: dict[str, list] = defaultdict(list)
    for r in runs:
        exp_id = (r.extra or {}).get("metadata", {}).get("experiment_id", "unknown")
        by_exp[exp_id].append(r)
    available = sorted(by_exp.keys())

    if arg_a and arg_b:
        branch_b, branch_a = arg_a, arg_b
    else:
        branch_b = os.environ.get("BRANCH_NAME", "").replace("/", "_")
        if not branch_b:
            non_main = [e for e in available if e != "main"]
            branch_b = non_main[-1] if non_main else ""
        branch_a = "main"

    if branch_a not in by_exp:
        print(f"No baseline '{branch_a}' in LangSmith. Available: {available}")
        sys.exit(0)
    if branch_b not in by_exp:
        print(f"No results for '{branch_b}'. Available: {available}")
        sys.exit(0)
    if branch_a == branch_b:
        print(f"Same experiment '{branch_a}', nothing to compare.")
        sys.exit(0)

    def summarize(runs: list) -> dict:
        results: dict = defaultdict(lambda: {"n": 0, "pass_rates": []})
        for r in runs:
            parts = (r.name or "").split("/")
            if len(parts) != 2:
                continue
            key = f"{parts[0]}/{parts[1]}"
            fb = {k: v.get("avg", 0) for k, v in (r.feedback_stats or {}).items()}
            results[key]["n"] += 1
            results[key]["pass_rates"].append(fb.get("checks_pass_rate", 0))
        for data in results.values():
            n = data["n"]
            data["pass_rate"] = sum(data["pass_rates"]) / n if n else 0
        return dict(results)

    summary_a = summarize(by_exp[branch_a])
    summary_b = summarize(by_exp[branch_b])
    print_comparison(branch_a, branch_b, summary_a, summary_b, len(by_exp[branch_a]), len(by_exp[branch_b]))


def main() -> None:
    arg_a = sys.argv[1] if len(sys.argv) >= 2 else None
    arg_b = sys.argv[2] if len(sys.argv) >= 3 else None

    looks_like_path = lambda s: "/" in s or s.startswith(".")
    if arg_a and arg_b and (looks_like_path(arg_a) or looks_like_path(arg_b)):
        compare_local(arg_a, arg_b)
    else:
        compare_langsmith(arg_a, arg_b)


if __name__ == "__main__":
    main()
