"""The harness × model cells of an eval run, declared in matrix.toml and picked with EVAL_MATRIX.

Usage: python -m scaffold.matrix   (prints the selected cells: name, agent, model, harbor args; tab-separated)
"""

from __future__ import annotations

import os
import sys
import tomllib
from pathlib import Path

MATRIX_FILE = Path(__file__).resolve().parent.parent / "matrix.toml"


def selected_cells(selection: str = "", env: dict | None = None, path: Path = MATRIX_FILE) -> list[dict]:
    """Cells picked by `selection`: empty for the defaults, `all`, or comma-separated names."""
    env = os.environ if env is None else env
    cells = tomllib.loads(path.read_text())["cell"]
    names = [n.strip() for n in selection.split(",") if n.strip()]
    if unknown := set(names) - {"all"} - {c["name"] for c in cells}:
        raise SystemExit(f"unknown cell(s) in EVAL_MATRIX: {', '.join(sorted(unknown))}")
    picked = []
    for cell in cells:
        if not (cell["name"] in names or "all" in names or (not names and cell.get("default"))):
            continue
        if missing := [v for v in cell.get("requires", []) if not env.get(v)]:
            if cell["name"] in names:
                raise SystemExit(f"cell {cell['name']} needs {', '.join(missing)}")
            print(f"skipping cell {cell['name']}: {', '.join(missing)} not set", file=sys.stderr)
            continue
        picked.append(cell)
    return picked


if __name__ == "__main__":
    for c in selected_cells(os.environ.get("EVAL_MATRIX", "")):
        print(c["name"], c["agent"], c["model"], " ".join(c.get("args", [])), sep="\t")
