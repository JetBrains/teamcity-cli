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

projects = tc("project", "list", "--json", "--limit", "0")["project"]
answer = {"top_level_projects": [p["id"] for p in projects if p.get("parentProjectId") == "_Root"],
          "shop_subprojects": [p["id"] for p in projects if p.get("parentProjectId") == "Shop"],
          "agent_pools": len(tc("pool", "list", "--json")["agentPool"]),
          "agents": len(tc("agent", "list", "--json", "--limit", "0")["agent"])}

json.dump(answer, open("answer.json", "w"), indent=1)
EOF
