#!/usr/bin/env python3
"""Statistical gate and report for skill eval runs.

Reads the run artifacts one eval session wrote to `results/<experiment>/` and
computes the only number that matters: the paired skill lift
(CURRENT − CONTROL per task, deterministic checks only) with a bootstrap 95% CI,
plus pass^k reliability and an advisory judge summary.

The gate is self-contained — both arms run in the same session, so live-server
drift cancels out and no cross-build baseline is needed:

  BLOCK when the lift CI lower bound < GATE_LIFT_FLOOR  (skill stopped helping)
  BLOCK when a guardrail task (single-arm, e.g. negative-unrelated) fails on
        half or more of its reps                        (skill misfires)

Usage:
    uv run python scripts/compare.py                       # results/$BRANCH_NAME (or newest)
    uv run python scripts/compare.py results/main          # explicit experiment
    uv run python scripts/compare.py results/A results/B   # informational A/B diff

Env:
    GATE_MODE        warn (default) — report a breach but exit 0
                     enforce        — exit 1 on a breach
    GATE_LIFT_FLOOR  CI lower-bound threshold, default -0.05 (-5 points)
    GATE_MIN_TASKS   paired tasks required before the gate applies, default 3

Exit codes: 0 ok / 1 gate breach (enforce only) / 2 bad input.
"""

from __future__ import annotations

import json
import math
import os
import random
import sys
from collections import defaultdict
from pathlib import Path
from statistics import mean

EVALS_ROOT = Path(__file__).resolve().parent.parent
RESULTS_ROOT = EVALS_ROOT / "results"
TASKS_FILE = EVALS_ROOT / "tasks.json"

BOOTSTRAP_ITERS = 10_000
BOOTSTRAP_SEED = 1337
PASS_K = 2

# Pre-fix artifacts carried a treatment-only freebie and judge grades blended
# into the check list; strip them so legacy runs are comparable to honest ones.
LEGACY_FREEBIE = "Skill loaded via treatment"
LEGACY_LLM_PREFIX = "[LLM]"


# ---------------------------------------------------------------------------
# Loading
# ---------------------------------------------------------------------------

def load_runs(exp_dir: Path) -> list[dict]:
    """One dict per run: task, treatment, deterministic rate, all-pass flag, grades."""
    runs = []
    for f in sorted(exp_dir.glob("*.json")):
        if f.name == "gate_summary.json":
            continue
        try:
            data = json.loads(f.read_text())
        except json.JSONDecodeError:
            continue
        results = data.get("results")
        if not isinstance(results, list):
            continue

        task = data.get("task_id")
        treatment = data.get("treatment")
        if not task or not treatment:  # legacy artifact: "task" is "name/TREATMENT"
            label = data.get("task", "")
            if "/" not in label:
                continue
            task, treatment = label.rsplit("/", 1)

        checks = [
            r for r in results
            if not r.get("message", "").startswith(LEGACY_LLM_PREFIX)
            and r.get("message") != LEGACY_FREEBIE
        ]
        if not checks:
            continue
        passed = sum(1 for r in checks if r.get("passed"))
        runs.append({
            "task": task,
            "treatment": treatment,
            "rate": passed / len(checks),
            "all_passed": passed == len(checks),
            "failed_checks": [r["check"] for r in checks if not r.get("passed")],
            "llm_grades": data.get("llm_grades") or [],
        })
    return runs


def paired_task_names() -> set[str]:
    """Tasks whose declared treatments include both arms (guardrails excluded)."""
    try:
        tasks = json.loads(TASKS_FILE.read_text())
    except OSError:
        return set()
    return {
        t["id"] for t in tasks
        if {"CONTROL", "CURRENT"} <= set(t.get("treatments", ["CONTROL", "CURRENT"]))
    }


# ---------------------------------------------------------------------------
# Statistics
# ---------------------------------------------------------------------------

def rates_by_task(runs: list[dict]) -> dict[str, dict[str, list[float]]]:
    out: dict[str, dict[str, list[float]]] = defaultdict(lambda: defaultdict(list))
    for r in runs:
        out[r["task"]][r["treatment"]].append(r["rate"])
    return out


def paired_lifts(by_task: dict) -> dict[str, float]:
    return {
        t: mean(arms["CURRENT"]) - mean(arms["CONTROL"])
        for t, arms in sorted(by_task.items())
        if arms.get("CONTROL") and arms.get("CURRENT")
    }


def bootstrap_lift_ci(by_task: dict, paired: list[str]) -> tuple[float, float]:
    """Hierarchical bootstrap: resample tasks, then reps within each arm."""
    rng = random.Random(BOOTSTRAP_SEED)
    means = []
    for _ in range(BOOTSTRAP_ITERS):
        lifts = []
        for t in (paired[rng.randrange(len(paired))] for _ in paired):
            ctrl, curr = by_task[t]["CONTROL"], by_task[t]["CURRENT"]
            c = [ctrl[rng.randrange(len(ctrl))] for _ in ctrl]
            u = [curr[rng.randrange(len(curr))] for _ in curr]
            lifts.append(mean(u) - mean(c))
        means.append(mean(lifts))
    means.sort()
    return (
        means[int(0.025 * BOOTSTRAP_ITERS)],
        means[min(int(0.975 * BOOTSTRAP_ITERS), BOOTSTRAP_ITERS - 1)],
    )


def pass_k(all_passed: list[bool], k: int = PASS_K) -> float | None:
    """Unbiased estimator of P(k random reps all pass): C(c,k)/C(n,k)."""
    n, c = len(all_passed), sum(all_passed)
    if n < k:
        return None
    return math.comb(c, k) / math.comb(n, k)


def judge_summary(runs: list[dict]) -> dict[str, dict[str, float]]:
    """dimension → arm → mean judge score (advisory)."""
    scores: dict[str, dict[str, list[float]]] = defaultdict(lambda: defaultdict(list))
    for r in runs:
        for g in r["llm_grades"]:
            scores[g["dimension"]][r["treatment"]].append(g["score"])
    return {
        dim: {arm: round(mean(v), 2) for arm, v in arms.items()}
        for dim, arms in scores.items()
    }


# ---------------------------------------------------------------------------
# Report + gate
# ---------------------------------------------------------------------------

def analyze(exp_dir: Path) -> dict | None:
    runs = load_runs(exp_dir)
    if not runs:
        print(f"  No run artifacts in {exp_dir}")
        return None

    by_task = rates_by_task(runs)
    declared_paired = paired_task_names()
    lifts = {t: l for t, l in paired_lifts(by_task).items()
             if not declared_paired or t in declared_paired}
    guardrails = {
        t: arms for t, arms in by_task.items()
        if t not in lifts and (not declared_paired or t not in declared_paired)
    }

    all_passed: dict[tuple[str, str], list[bool]] = defaultdict(list)
    for r in runs:
        all_passed[(r["task"], r["treatment"])].append(r["all_passed"])

    print(f"\n  Experiment: {exp_dir.name}  ({len(runs)} runs)\n")
    header = (f"  {'Task':<28s} {'CONTROL':>8s} {'CURRENT':>8s} {'Lift':>7s} "
              f"{'pass^' + str(PASS_K) + ' C/T':>12s}")
    print(header)
    print(f"  {'-' * (len(header) - 2)}")
    for t in sorted(by_task):
        arms = by_task[t]
        ctrl = mean(arms["CONTROL"]) if arms.get("CONTROL") else None
        curr = mean(arms["CURRENT"]) if arms.get("CURRENT") else None
        pk_c = pass_k(all_passed[(t, "CONTROL")])
        pk_u = pass_k(all_passed[(t, "CURRENT")])
        fmt = lambda v, pct=True: ("    -" if v is None else f"{v:>5.0%}" if pct else f"{v:.2f}")
        lift_s = f"{lifts[t]:>+6.0%}" if t in lifts else ("guard" if t in guardrails else "     -")
        print(f"  {t:<28s} {fmt(ctrl):>8s} {fmt(curr):>8s} {lift_s:>7s} "
              f"{fmt(pk_c):>5s}/{fmt(pk_u)}")

    summary: dict = {"experiment": exp_dir.name, "runs": len(runs),
                     "per_task_lift": {t: round(l, 3) for t, l in lifts.items()}}

    if lifts:
        lift_mean = mean(lifts.values())
        ci_low, ci_high = bootstrap_lift_ci(by_task, list(lifts))
        print(f"\n  Paired skill lift: {lift_mean:+.1%}  "
              f"(95% CI [{ci_low:+.1%}, {ci_high:+.1%}], {len(lifts)} tasks)")
        summary.update(lift=round(lift_mean, 4), ci_low=round(ci_low, 4),
                       ci_high=round(ci_high, 4))

    judges = judge_summary(runs)
    if judges:
        print("\n  Judge (advisory, 1-5):")
        for dim, arms in sorted(judges.items()):
            arm_s = "  ".join(f"{a}={v}" for a, v in sorted(arms.items()))
            print(f"    {dim:<24s} {arm_s}")
        summary["judge"] = judges

    breaches = []
    floor = float(os.environ.get("GATE_LIFT_FLOOR", "-0.05"))
    min_tasks = int(os.environ.get("GATE_MIN_TASKS", "3"))
    if len(lifts) >= min_tasks:
        if summary["ci_low"] < floor:
            breaches.append(
                f"lift CI lower bound {summary['ci_low']:+.1%} < floor {floor:+.1%}"
            )
    else:
        print(f"\n  Gate skipped: {len(lifts)} paired task(s) < GATE_MIN_TASKS={min_tasks}")

    for t, arms in sorted(guardrails.items()):
        flags = [ap for (task, _), aps in all_passed.items() if task == t for ap in aps]
        if flags and sum(not f for f in flags) * 2 >= len(flags):
            failed = {c for r in runs if r["task"] == t for c in r["failed_checks"]}
            breaches.append(f"guardrail '{t}' failed {sum(not f for f in flags)}/{len(flags)} reps "
                            f"({', '.join(sorted(failed))})")

    summary["breaches"] = breaches
    (exp_dir / "gate_summary.json").write_text(json.dumps(summary, indent=2))
    return summary


def gate(summary: dict) -> int:
    mode = os.environ.get("GATE_MODE", "warn").lower()
    if not summary["breaches"]:
        print("\n  GATE: ok")
        return 0
    verdict = "BLOCK" if mode == "enforce" else "WOULD BLOCK (GATE_MODE=warn)"
    print(f"\n  GATE: {verdict}")
    for b in summary["breaches"]:
        print(f"    - {b}")
    return 1 if mode == "enforce" else 0


def diff(dir_a: Path, dir_b: Path) -> int:
    """Informational: compare two experiments' lifts."""
    a, b = analyze(dir_a), analyze(dir_b)
    if not a or not b:
        return 2
    if "lift" in a and "lift" in b:
        print(f"\n  Lift {dir_a.name} → {dir_b.name}: "
              f"{a['lift']:+.1%} → {b['lift']:+.1%}  (Δ {b['lift'] - a['lift']:+.1%})")
    return 0


def resolve_experiment_dir() -> Path | None:
    branch = os.environ.get("BRANCH_NAME", "").replace("/", "_")
    if branch and (RESULTS_ROOT / branch).is_dir():
        return RESULTS_ROOT / branch
    dirs = [d for d in RESULTS_ROOT.iterdir() if d.is_dir()] if RESULTS_ROOT.is_dir() else []
    return max(dirs, key=lambda d: d.stat().st_mtime) if dirs else None


def main() -> int:
    args = [Path(a) for a in sys.argv[1:]]
    for a in args:
        if not a.is_dir():
            print(f"  Not a directory: {a}")
            return 2

    if len(args) == 2:
        return diff(args[0], args[1])
    exp_dir = args[0] if args else resolve_experiment_dir()
    if not exp_dir:
        print("  No experiment directory found — pass one, e.g. scripts/compare.py results/main")
        return 2
    summary = analyze(exp_dir)
    if summary is None:
        return 2
    return gate(summary)


if __name__ == "__main__":
    sys.exit(main())
