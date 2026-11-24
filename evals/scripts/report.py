"""Write a static report.html for an experiment, for a TeamCity report tab.

Usage: report.py <results-dir>   (run compare.py on the same directory first)
"""

from __future__ import annotations

import html
import json
import sys
from collections import Counter, defaultdict
from pathlib import Path

from compare import cell_dirs

PAGE = """<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Skill eval: {exp}</title>
<style>
:root {{ color-scheme: light dark; --fg: #1f2328; --bg: #fff; --muted: #656d76; --line: #d0d7de; --up: #1a7f37; --down: #cf222e; }}
@media (prefers-color-scheme: dark) {{ :root {{ --fg: #e6edf3; --bg: #0d1117; --muted: #8d96a0; --line: #30363d; --up: #3fb950; --down: #f85149; }} }}
body {{ font: 14px/1.5 system-ui, sans-serif; color: var(--fg); background: var(--bg); margin: 24px; max-width: 1100px; }}
table {{ border-collapse: collapse; width: 100%; margin: 8px 0 24px; }}
th, td {{ text-align: left; padding: 6px 10px; border-bottom: 1px solid var(--line); vertical-align: top; }}
td.n {{ text-align: right; font-variant-numeric: tabular-nums; }}
.up {{ color: var(--up); }} .down {{ color: var(--down); }} .muted {{ color: var(--muted); }}
</style></head><body>
<h1>Skill eval: {exp}</h1>
<h2>Harness × model</h2>
<p class="muted">Outcome score over paired tasks. Tokens, turns and seconds are means per run, without / with skill.</p>
<table><tr><th>Harness</th><th>Model</th><th>Without skill</th><th>With skill</th><th>Paired lift</th><th>95% CI</th>
<th>Skill invoked</th><th>Tokens</th><th>Turns</th><th>Seconds</th><th>Gate</th></tr>{matrix_rows}</table>
{cells}
</body></html>
"""

CELL = """<h2>{title}</h2>
<p>{runs} runs, {invalid} excluded as invalid. Paired skill lift on the outcome score <b>{lift}</b> (95% CI {ci}).{cost} Gate: {gate}.</p>
<table><tr><th>Task</th><th>Lift</th><th>Missed without skill</th><th>Missed with skill</th></tr>{task_rows}</table>
<table><tr><th>Efficiency per run</th><th>Without skill</th><th>With skill</th><th>Paired change</th><th>95% CI</th></tr>{eff_rows}</table>
"""


def pct(x: float) -> str:
    return f"{x:+.0%}"


def lift_cell(v: float) -> str:
    return f'<td class="n {"up" if v > 0 else "down" if v < 0 else ""}">{pct(v)}</td>'


def gate_text(summary: dict) -> str:
    breaches = summary.get("breaches") or []
    return html.escape("; ".join(breaches)) if breaches else "ok"


def matrix_row(summary: dict) -> str:
    """One harness × model row of the matrix table."""
    def pair(key: str, fmt: str) -> str:
        arms = summary.get("efficiency", {}).get(key, {})
        return " / ".join(format(arms[a], fmt) if a in arms else "–" for a in ("CONTROL", "CURRENT"))

    used = summary.get("skill_invoked", {}).get("CURRENT")
    has_lift = "lift" in summary
    return (
        f"<tr><td>{html.escape(summary.get('agent') or '–')}</td>"
        f"<td>{html.escape(summary.get('model') or summary.get('experiment', ''))}</td>"
        + (f'<td class="n">{summary["control"]:.0%}</td><td class="n">{summary["current"]:.0%}</td>'
           f'{lift_cell(summary["lift"])}<td class="n">{pct(summary["ci_low"])} to {pct(summary["ci_high"])}</td>'
           if has_lift else '<td class="n muted" colspan="4">no paired tasks</td>')
        + f'<td class="n">{"–" if used is None else format(used, ".0%")}</td>'
        f'<td class="n">{pair("total_tokens", ",.0f")}</td><td class="n">{pair("num_turns", ".1f")}</td>'
        f'<td class="n">{pair("duration_sec", ".0f")}</td><td>{gate_text(summary)}</td></tr>'
    )


def cell_section(cell_dir: Path, summary: dict) -> str:
    """The per-task and efficiency tables of one cell."""
    failed: dict[tuple[str, str], Counter] = defaultdict(Counter)
    for f in cell_dir.glob("*.json"):
        data = json.loads(f.read_text())
        if not isinstance(data, dict):
            continue
        rows = data.get("results") or []
        if "reward" in data:  # outcome-graded: show missed answer fields and tripped safety gates
            rows = [dict(r, check=f"{kind}:{r['check']}") for kind in ("outcome", "safety") for r in data[kind]["results"]]
        for r in rows:
            if not r.get("passed"):
                failed[(data.get("task_id"), data.get("treatment"))][r["check"]] += 1

    def fails(task: str, arm: str) -> str:
        c = failed.get((task, arm))
        return ", ".join(f"{html.escape(k)} ×{v}" for k, v in c.most_common()) if c else '<span class="muted">none</span>'

    task_rows = "".join(
        f"<tr><td>{html.escape(t)}</td>{lift_cell(v)}"
        f"<td>{fails(t, 'CONTROL')}</td><td>{fails(t, 'CURRENT')}</td></tr>"
        for t, v in sorted(summary.get("per_task_lift", {}).items(), key=lambda kv: -kv[1])
    )
    changes = summary.get("efficiency_change", {})

    def change_cells(key: str) -> str:
        c = changes.get(key)
        if not c:
            return '<td class="n muted">–</td><td class="n muted">–</td>'
        # fewer tokens, turns, seconds and dollars is the improvement
        return (f'<td class="n {"up" if c["change"] < 0 else "down" if c["change"] > 0 else ""}">{pct(c["change"])}</td>'
                f'<td class="n">{pct(c["ci_low"])} to {pct(c["ci_high"])}</td>')

    eff_rows = "".join(
        f'<tr><td>{html.escape(k)}</td><td class="n">{arms.get("CONTROL", 0):,.4g}</td>'
        f'<td class="n">{arms.get("CURRENT", 0):,.4g}</td>{change_cells(k)}</tr>'
        for k, arms in summary.get("efficiency", {}).items()
    )
    headline = next(((label, changes[k]) for k, label in (("cost_usd", "Cost"), ("total_tokens", "Tokens")) if k in changes), None)
    cost = (f" {headline[0]} per run with the skill <b>{pct(headline[1]['change'])}</b> "
            f"(95% CI {pct(headline[1]['ci_low'])} to {pct(headline[1]['ci_high'])})." if headline else "")
    title = " on ".join(filter(None, (summary.get("agent"), summary.get("model")))) or "Per task"
    invalid_file = cell_dir / "invalid_trials.json"
    return CELL.format(
        invalid=len(json.loads(invalid_file.read_text())) if invalid_file.is_file() else 0,
        title=html.escape(title), runs=summary.get("runs", 0), lift=pct(summary.get("lift", 0)),
        ci=f"{pct(summary.get('ci_low', 0))} to {pct(summary.get('ci_high', 0))}",
        gate=gate_text(summary), task_rows=task_rows, eff_rows=eff_rows, cost=cost,
    )


def main(exp_dir: Path) -> None:
    cells = [(d, json.loads((d / "gate_summary.json").read_text()))
             for d in cell_dirs(exp_dir) if (d / "gate_summary.json").is_file()]
    out = exp_dir / "report.html"
    out.write_text(PAGE.format(
        exp=html.escape(exp_dir.name),
        matrix_rows="".join(matrix_row(s) for _, s in cells),
        cells="".join(cell_section(d, s) for d, s in cells),
    ))
    print(f"Wrote {out}")


if __name__ == "__main__":
    main(Path(sys.argv[1]))
