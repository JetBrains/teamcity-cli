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

shop_jobs = {j["id"] for j in tc("api", "/app/rest/buildTypes?locator=affectedProject:(id:Shop)&fields=buildType(id)")["buildType"]}
queue = [b for b in tc("api", "/app/rest/buildQueue?fields=build(id,buildTypeId,queuedDate,triggered(user(username)),"
                       "compatibleAgents(count))")["build"] if b["buildTypeId"] in shop_jobs]
oldest = min(queue, key=lambda b: b["queuedDate"])
requirement = tc("api", f"/app/rest/buildTypes/id:{oldest['buildTypeId']}/agent-requirements")["agent-requirement"][0]
answer = {"job": oldest["buildTypeId"], "triggered_by": oldest["triggered"]["user"]["username"],
          "missing_agent_parameter": {p["name"]: p["value"] for p in requirement["properties"]["property"]}["property-name"],
          "compatible_agents": oldest["compatibleAgents"]["count"], "queued_runs_in_project": len(queue)}

json.dump(answer, open("answer.json", "w"), indent=1)
EOF
