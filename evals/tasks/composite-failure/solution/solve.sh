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

def dependencies(run_id):
    return runs(f"snapshotDependency:(to:(id:{run_id}),includeInitial:false),defaultFilter:false", 100)


failed_runs = []
for run in runs("buildType:(id:Shop_Release_Verify),status:FAILURE", 100):
    failed = [d for d in dependencies(run["id"]) if d["status"] == "FAILURE" and not d.get("failedToStart")]
    failed_ids = {d["id"] for d in failed}
    roots = [d for d in failed if not failed_ids & {x["id"] for x in dependencies(d["id"])}]
    failed_runs.append({"number": run["number"], "branch": run["branchName"],
                        "root_cause_jobs": [d["buildTypeId"] for d in roots],
                        "failed_tests": sorted({t for d in roots for t in failed_tests(d["id"])})})
answer = {"failed_runs": failed_runs}

json.dump(answer, open("answer.json", "w"), indent=1)
EOF
