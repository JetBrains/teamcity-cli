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

subprojects = []
for project in tc("api", "/app/rest/projects/id:Shop?fields=projects(project(id))")["projects"]["project"]:
    jobs = tc("api", f"/app/rest/buildTypes?locator=project:(id:{project['id']})&fields=buildType(id)")["buildType"]
    never_run = failing = 0
    for job in jobs:
        never_run += not runs(f"buildType:(id:{job['id']})", 1)
        latest = tc("api", f"/app/rest/builds?locator=buildType:(id:{job['id']}),state:finished,count:1&fields=build(status)")["build"]
        failing += bool(latest) and latest[0]["status"] == "FAILURE"
    subprojects.append({"project": project["id"], "jobs": len(jobs), "never_run": never_run, "failing": failing})
answer = {"subprojects": subprojects}

json.dump(answer, open("answer.json", "w"), indent=1)
EOF
