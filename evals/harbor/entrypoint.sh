#!/bin/sh
# Expand ${VAR} references in seeded workspace files (e.g. teamcity.toml) from the task's [environment.env].
python3 - <<'PY'
import os, pathlib
for p in pathlib.Path("/app").rglob("*"):
    if p.is_file() and "${" in (text := p.read_text(errors="ignore")):
        p.write_text(os.path.expandvars(text))
PY
exec "$@"
