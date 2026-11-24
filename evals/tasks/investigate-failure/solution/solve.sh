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

on_main = runs("buildType:(id:Shop_Backend_UnitTests),branch:(name:main)", 1000)
first_failing = next(r for i, r in enumerate(on_main) if on_main[i + 1]["status"] == "SUCCESS")
culprit = tc("run", "changes", str(first_failing["id"]), "--json")["change"][0]
answer = {"failed_tests": failed_tests(on_main[0]["id"]), "first_failing_run": first_failing["number"],
          "culprit_commit": culprit["version"], "culprit_author": culprit["username"]}

json.dump(answer, open("answer.json", "w"), indent=1)
EOF
