"""Locate the teamcity-cli skill the CLI under test ships.

The skill lives in JetBrains/teamcity-skills and reaches this repository as a Go
module dependency, so it is resolved through `go list` from the version go.mod
pins: a dependency bump changes what the CURRENT arm measures.

Usage: python -m scaffold.skill   (prints the skill directory)
"""

from __future__ import annotations

import subprocess
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SKILLS_MODULE = "github.com/JetBrains/teamcity-skills"


def _module_dir() -> str:
    out = subprocess.run(["go", "list", "-m", "-f", "{{.Dir}}", SKILLS_MODULE],
                         cwd=REPO_ROOT, capture_output=True, text=True, timeout=120)
    return out.stdout.strip() if out.returncode == 0 else ""


def skill_dir() -> Path:
    """The skill directory in the module cache; downloads the module on a fresh checkout."""
    module_dir = _module_dir()
    if not module_dir:  # pinned but not yet extracted, the normal state of a fresh checkout
        subprocess.run(["go", "mod", "download", SKILLS_MODULE],
                       cwd=REPO_ROOT, capture_output=True, text=True, timeout=300)
        module_dir = _module_dir()
    if not module_dir:
        raise RuntimeError(f"cannot locate {SKILLS_MODULE}; without it CURRENT would silently equal CONTROL")
    return Path(module_dir) / "skills" / "teamcity-cli"


if __name__ == "__main__":
    print(skill_dir())
