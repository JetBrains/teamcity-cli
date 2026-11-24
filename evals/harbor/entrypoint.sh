#!/bin/sh
# Expand ${TEAMCITY_URL} in workspace files to the server the CLI config (baked into the image) names.
python3 - <<'PY'
import os, pathlib, re
cfg = pathlib.Path.home() / ".config" / "tc" / "config.yml"
found = re.search(r"^default_server: (\S+)", cfg.read_text() if cfg.is_file() else "", re.M)
os.environ["TEAMCITY_URL"] = found.group(1) if found else ""
for p in pathlib.Path("/app").rglob("*"):
    if p.is_file() and "${" in (text := p.read_text(errors="ignore")):
        p.write_text(os.path.expandvars(text))
PY
exec "$@"
