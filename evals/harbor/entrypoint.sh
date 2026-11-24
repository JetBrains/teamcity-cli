#!/bin/sh
# Log the CLI in from the task's [environment.env] the way most users are: token in the CLI
# config, not in TEAMCITY_* env vars. Then expand ${TEAMCITY_URL} in seeded workspace files.
python3 - <<'PY'
import os, pathlib
url, token = os.environ.get("TC_EVAL_URL", ""), os.environ.get("TC_EVAL_TOKEN", "")
if url and token:
    cfg = pathlib.Path.home() / ".config" / "tc" / "config.yml"
    cfg.parent.mkdir(parents=True, exist_ok=True)
    cfg.write_text(f"analytics_notice_shown: true\ndefault_server: {url}\nservers:\n    {url}:\n        token: {token}\n")
    cfg.chmod(0o600)
os.environ["TEAMCITY_URL"] = url
for p in pathlib.Path("/app").rglob("*"):
    if p.is_file() and "${" in (text := p.read_text(errors="ignore")):
        p.write_text(os.path.expandvars(text))
PY
exec "$@"
