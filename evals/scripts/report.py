"""Write a static report.html for an experiment, for a TeamCity report tab.

Usage: report.py <results-dir>   (run compare.py on the same directory first)
"""

from __future__ import annotations

import html
import json
import sys
from collections import Counter, defaultdict
from pathlib import Path

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
<p>{runs} runs. Paired skill lift <b>{lift}</b> (95% CI {ci}). Gate: {gate}.</p>
<h2>Per task</h2>
<table><tr><th>Task</th><th>Lift</th><th>Failed checks without skill</th><th>Failed checks with skill</th></tr>{task_rows}</table>
<h2>Efficiency per run</h2>
<table><tr><th>Metric</th><th>Without skill</th><th>With skill</th></tr>{eff_rows}</table>
</body></html>
"""


def pct(x: float) -> str:
    return f"{x:+.0%}"


def main(exp_dir: Path) -> None:
    summary = json.loads((exp_dir / "gate_summary.json").read_text())
    failed: dict[tuple[str, str], Counter] = defaultdict(Counter)
    for f in exp_dir.glob("*.json"):
        data = json.loads(f.read_text())
        for r in data.get("results") or []:
            if not r.get("passed"):
                failed[(data.get("task_id"), data.get("treatment"))][r["check"]] += 1

    def fails(task: str, arm: str) -> str:
        c = failed.get((task, arm))
        return ", ".join(f"{html.escape(k)} ×{v}" for k, v in c.most_common()) if c else '<span class="muted">none</span>'

    task_rows = "".join(
        f'<tr><td>{html.escape(t)}</td><td class="n {"up" if v > 0 else "down" if v < 0 else ""}">{pct(v)}</td>'
        f"<td>{fails(t, 'CONTROL')}</td><td>{fails(t, 'CURRENT')}</td></tr>"
        for t, v in sorted(summary.get("per_task_lift", {}).items(), key=lambda kv: -kv[1])
    )
    eff_rows = "".join(
        f'<tr><td>{html.escape(k)}</td><td class="n">{arms.get("CONTROL", 0):,.1f}</td><td class="n">{arms.get("CURRENT", 0):,.1f}</td></tr>'
        for k, arms in summary.get("efficiency", {}).items()
    )
    breaches = summary.get("breaches") or []
    out = exp_dir / "report.html"
    out.write_text(PAGE.format(
        exp=html.escape(summary.get("experiment", exp_dir.name)), runs=summary.get("runs", 0),
        lift=pct(summary.get("lift", 0)), ci=f"{pct(summary.get('ci_low', 0))} to {pct(summary.get('ci_high', 0))}",
        gate=html.escape("; ".join(breaches)) if breaches else "ok", task_rows=task_rows, eff_rows=eff_rows,
    ))
    print(f"Wrote {out}")


if __name__ == "__main__":
    main(Path(sys.argv[1]))
