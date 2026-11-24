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

jobs = tc("api", "/app/rest/buildTypes?locator=affectedProject:(id:Shop)&fields=buildType(id,steps(step(properties(property(name,value)))))")["buildType"]
images, with_docker = {}, 0
for job in jobs:
    used = {p["value"] for step in job["steps"].get("step", []) for p in step["properties"]["property"]
            if p["name"] == "plugin.docker.imageId"}
    with_docker += bool(used)
    for image in used:
        images[image] = images.get(image, 0) + 1
answer = {"jobs_with_docker_steps": with_docker, "images": images}

json.dump(answer, open("answer.json", "w"), indent=1)
EOF
