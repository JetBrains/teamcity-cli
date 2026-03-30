#!/usr/bin/env python3
"""Compare eval experiments via LangSmith.

Queries LangSmith for runs tagged with experiment_id (= branch name in CI).
Compares current branch against main automatically.

Regression gate: fails if the OVERALL average of CURRENT runs drops >10%.
Individual task swings are reported but don't gate — single-run variance is too high.

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
    runs = list(client.list_runs(project_name=project, limit=100, is_root=True))
    by_exp: dict[str, list] = defaultdict(list)
    for r in runs:
        exp_id = (r.extra or {}).get("metadata", {}).get("experiment_id", "unknown")
        by_exp[exp_id].append(r)
    return by_exp


def summarize(runs: list) -> dict[str, dict]:
    results: dict[str, dict] = defaultdict(lambda: {"n": 0, "pass_rates": []})
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


def avg(values: list[float]) -> float:
    return sum(values) / len(values) if values else 0


def main():
    client = Client()
    project = os.environ.get("LANGSMITH_PROJECT", "teamcity-cli")

    by_exp = get_runs_by_experiment(client, project)
    available = sorted(by_exp.keys())

    if len(sys.argv) == 3:
        branch_b, branch_a = sys.argv[1], sys.argv[2]
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

    summary_a = summarize(by_exp[branch_a])
    summary_b = summarize(by_exp[branch_b])
    all_keys = sorted(set(list(summary_a.keys()) + list(summary_b.keys())))

    print(f"\n  {branch_a} ({len(by_exp[branch_a])} runs) → {branch_b} ({len(by_exp[branch_b])} runs)\n")
    print(f"  {'Task/Treatment':<45s}  {branch_a:>7s}  {branch_b:>7s}  {'Delta':>7s}")
    print(f"  {'-'*70}")

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
            print(f"  {key:<45s}  {pa:>6.0%}  {pb:>6.0%}   {delta:>+5.0%}{marker}")
            if is_current:
                current_deltas.append(delta)
        elif b:
            print(f"  {key:<45s}  {'  -':>7s}  {pb:>6.0%}    new")
        else:
            print(f"  {key:<45s}  {pa:>6.0%}  {'  -':>7s}   gone")

    print(f"  {'-'*70}")
    if current_deltas:
        overall = avg(current_deltas)
        print(f"  {'CURRENT AVERAGE':<45s}          {' ':>7s}  {overall:>+5.0%}")
        print()

        if overall < -0.10:
            print(f"  REGRESSION: overall CURRENT average dropped {overall:+.0%} (threshold: -10%)")
            sys.exit(1)
        else:
            print(f"  No regression (overall CURRENT: {overall:+.0%})")
    else:
        print()
        print(f"  No CURRENT runs to compare.")


if __name__ == "__main__":
    main()
