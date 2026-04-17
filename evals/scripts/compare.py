#!/usr/bin/env python3
"""Compare eval experiments via Sentry or local result directories.

Sentry path (preferred): queries the Discover events API for transactions tagged
with experiment_id (= branch name). Requires SENTRY_ORG + SENTRY_AUTH_TOKEN —
the DSN is ingest-only and cannot read.

Local path (fallback): diffs two `results/<experiment_id>/` directories written
by the test runner. Works with zero credentials.

Regression gate: fails if the OVERALL average of CURRENT runs drops >10%.
Individual task swings are reported but don't gate — single-run variance is too high.

Usage:
    uv run scripts/compare.py                          # current branch vs main (Sentry)
    uv run scripts/compare.py feature_x main           # explicit A vs B (Sentry)
    uv run scripts/compare.py results/eval-X results/eval-Y  # local directories
"""

from __future__ import annotations

import json
import os
import sys
import urllib.parse
import urllib.request
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


def _sentry_query(org: str, token: str, query: str, fields: list[str], stats_period: str = "30d") -> list[dict]:
    """Hit Sentry's Discover events API. Returns the `data` array."""
    host = os.environ.get("SENTRY_HOST", "sentry.io")
    params: list[tuple[str, str]] = [
        ("query", query),
        ("statsPeriod", stats_period),
        ("per_page", "100"),
        ("dataset", "transactions"),
        ("referrer", "teamcity-cli-evals"),
    ]
    for f in fields:
        params.append(("field", f))
    url = f"https://{host}/api/0/organizations/{org}/events/?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    with urllib.request.urlopen(req, timeout=30) as resp:
        payload = json.loads(resp.read().decode())
    return payload.get("data", [])


def compare_sentry(arg_a: str | None, arg_b: str | None) -> None:
    org = os.environ.get("SENTRY_ORG")
    token = os.environ.get("SENTRY_AUTH_TOKEN")
    if not org or not token:
        print("Sentry compare needs SENTRY_ORG and SENTRY_AUTH_TOKEN.")
        sys.exit(1)

    if arg_a and arg_b:
        branch_b, branch_a = arg_a, arg_b
    else:
        branch_b = os.environ.get("BRANCH_NAME", "").replace("/", "_")
        branch_a = "main"
        if not branch_b:
            print("Set BRANCH_NAME or pass two experiment IDs.")
            sys.exit(0)
    if branch_a == branch_b:
        print(f"Same experiment '{branch_a}', nothing to compare.")
        sys.exit(0)

    fields = ["tags[task]", "tags[treatment]", "avg(measurements.pass_rate)", "count()"]

    def summarize(experiment_id: str) -> tuple[dict, int]:
        rows = _sentry_query(
            org,
            token,
            query=f"event.type:transaction tags[experiment_id]:{experiment_id}",
            fields=fields,
        )
        summary: dict[str, dict] = {}
        total = 0
        for row in rows:
            task = row.get("tags[task]") or ""
            treat = row.get("tags[treatment]") or ""
            if not task or not treat:
                continue
            key = f"{task}/{treat}"
            n = int(row.get("count()", 0) or 0)
            summary[key] = {
                "pass_rate": float(row.get("avg(measurements.pass_rate)", 0) or 0),
                "n": n,
            }
            total += n
        return summary, total

    summary_a, count_a = summarize(branch_a)
    summary_b, count_b = summarize(branch_b)
    if not summary_a:
        print(f"No baseline '{branch_a}' in Sentry.")
        sys.exit(0)
    if not summary_b:
        print(f"No results for '{branch_b}' in Sentry.")
        sys.exit(0)
    print_comparison(branch_a, branch_b, summary_a, summary_b, count_a, count_b)


def main() -> None:
    arg_a = sys.argv[1] if len(sys.argv) >= 2 else None
    arg_b = sys.argv[2] if len(sys.argv) >= 3 else None

    looks_like_path = lambda s: "/" in s or s.startswith(".")
    if arg_a and arg_b and (looks_like_path(arg_a) or looks_like_path(arg_b)):
        compare_local(arg_a, arg_b)
    elif os.environ.get("SENTRY_AUTH_TOKEN") and os.environ.get("SENTRY_ORG"):
        compare_sentry(arg_a, arg_b)
    else:
        print("  No SENTRY_AUTH_TOKEN/SENTRY_ORG set — DSN alone can't read back.")
        print("  Pass two results/ paths for local diff, or set an auth token.")
        sys.exit(2)


if __name__ == "__main__":
    main()
