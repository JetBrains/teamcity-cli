#!/bin/sh
# Oracle: derives the reference answer from the server with the CLI, the way a careful agent would.
exec python3 - <<'EOF'
import json
import subprocess


def tc(*args):
    """Run `teamcity <args>` and parse its JSON output."""
    return json.loads(subprocess.run(["teamcity", *args], capture_output=True, text=True, check=True).stdout)


def runs(locator, count):
    """Finished runs matching a build locator, newest first; every branch unless the locator names one."""
    fields = "build(id,number,buildTypeId,status,branchName,failedToStart,triggered(type,user(username)))"
    branch = "" if "branch:" in locator else ",branch:default:any"
    return tc("api", f"/app/rest/builds?locator={locator}{branch},state:finished,count:{count}&fields={fields}")["build"]


def failed_tests(run_id):
    return [t["name"] for t in tc("run", "tests", str(run_id), "--failed", "--json", "--limit", "0").get("testOccurrence") or []]

import re

last = tc("run", "list", "--project", "Shop_Backend", "--limit", "50", "--json")["build"]
broken = runs("buildType:(id:Shop_Backend_Build),status:FAILURE", 1)[0]
log = subprocess.run(["teamcity", "run", "log", str(broken["id"]), "--raw"], capture_output=True, text=True).stdout
latest = runs("buildType:(id:Shop_Backend_UnitTests),branch:(name:main)", 1)[0]
tests = tc("run", "tests", str(latest["id"]), "--json", "--limit", "0")["testOccurrence"]
slowest = max(tests, key=lambda t: t.get("duration", 0))
answer = {"failed_runs": sum(r["status"] == "FAILURE" for r in last),
          "error_file": re.search(r"(\S+\.java):\d+: error", log).group(1),
          "slowest_test": slowest["name"], "slowest_test_ms": slowest["duration"]}

json.dump(answer, open("answer.json", "w"), indent=1)
EOF
