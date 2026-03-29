#!/usr/bin/env python3
"""Compare eval experiments via LangSmith.

Queries LangSmith for runs tagged with experiment_id (= branch name in CI).
Compares current branch against main automatically.

Usage:
    uv run scripts/compare.py                    # current branch vs main
    uv run scripts/compare.py feature_x main     # explicit A vs B
"""

from __future__ import annotations

import os
import sys
from collections import defaultdict

from langsmith import Client


def get_runs_by_experiment(client: Client, project: str) -> dict[str, list]:
    """Group root runs by experiment_id from metadata."""
    runs = list(client.list_runs(project_name=project, limit=100, is_root=True))
    by_exp: dict[str, list] = defaultdict(list)
    for r in runs:
        exp_id = (r.extra or {}).get("metadata", {}).get("experiment_id", "unknown")
        by_exp[exp_id].append(r)
    return by_exp


def summarize(runs: list) -> dict[str, dict]:
    results: dict[str, dict] = defaultdict(lambda: {"n": 0, "pass_rates": [], "durations": []})
    for r in runs:
        name = r.name or ""
        parts = name.split("/")
        if len(parts) != 2:
            continue
        task_treatment = f"{parts[0]}/{parts[1]}"
        fb = {k: v.get("avg", 0) for k, v in (r.feedback_stats or {}).items()}
        results[task_treatment]["n"] += 1
        results[task_treatment]["pass_rates"].append(fb.get("checks_pass_rate", 0))
        results[task_treatment]["durations"].append(fb.get("duration_seconds", 0))

    for data in results.values():
        n = data["n"]
        data["pass_rate"] = sum(data["pass_rates"]) / n if n else 0
        data["duration"] = sum(data["durations"]) / n if n else 0
    return dict(results)


def avg(values: list[float]) -> float:
    return sum(values) / len(values) if values else 0


def main():
    client = Client()
    project = os.environ.get("LANGSMITH_PROJECT", "teamcity-cli")

    by_exp = get_runs_by_experiment(client, project)
    available = sorted(by_exp.keys())

    # Determine what to compare
    if len(sys.argv) == 3:
        branch_b, branch_a = sys.argv[1], sys.argv[2]
    else:
        branch_b = os.environ.get("BRANCH_NAME", "").replace("/", "_")
        if not branch_b:
            # Pick most recent non-main experiment
            non_main = [e for e in available if e != "main"]
            branch_b = non_main[-1] if non_main else ""
        branch_a = "main"

    if branch_a not in by_exp:
        print(f"No baseline '{branch_a}' in LangSmith. Available: {available}")
        print("Run evals on main first to enable comparison.")
        sys.exit(0)

    if branch_b not in by_exp:
        print(f"No results for '{branch_b}' in LangSmith. Available: {available}")
        sys.exit(0)

    if branch_a == branch_b:
        print(f"Same experiment '{branch_a}', nothing to compare.")
        sys.exit(0)

    summary_a = summarize(by_exp[branch_a])
    summary_b = summarize(by_exp[branch_b])
    all_keys = sorted(set(list(summary_a.keys()) + list(summary_b.keys())))

    print(f"\n  {branch_a} ({len(by_exp[branch_a])} runs) → {branch_b} ({len(by_exp[branch_b])} runs)\n")
    print(f"  {'Task/Treatment':<45s}  {branch_a:>7s}  {branch_b:>7s}  {'Delta':>7s}")
    print(f"  {'-'*70}")

    deltas = []
    regressions = []
    for key in all_keys:
        a = summary_a.get(key, {})
        b = summary_b.get(key, {})
        pa = a.get("pass_rate", 0)
        pb = b.get("pass_rate", 0)

        if a and b:
            delta = pb - pa
            deltas.append(delta)
            marker = "  !!!" if delta < -0.05 else ""
            if delta < -0.05:
                regressions.append(key)
            print(f"  {key:<45s}  {pa:>6.0%}  {pb:>6.0%}   {delta:>+5.0%}{marker}")
        elif b:
            print(f"  {key:<45s}  {'  -':>7s}  {pb:>6.0%}    new")
        else:
            print(f"  {key:<45s}  {pa:>6.0%}  {'  -':>7s}   gone")

    if deltas:
        print(f"  {'-'*70}")
        print(f"  {'AVERAGE':<45s}          {' ':>7s}  {avg(deltas):>+5.0%}")

    print()
    if regressions:
        print(f"  REGRESSIONS: {', '.join(regressions)}")
        sys.exit(1)
    else:
        print(f"  No regressions.")


if __name__ == "__main__":
    main()
