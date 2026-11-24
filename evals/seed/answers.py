"""Ground truth for the seeded server, read back over REST rather than taken from the seed plan.

Writes manifest.json (facts about the fixture plus the reference answer of every task) and each
task's tests/expected.json, which Harbor only copies into a trial after the agent has finished.
The fingerprint is a hash of the facts; the snapshot image carries it as a label, so a run refuses
a snapshot whose contents differ from the ones the checked-in references were read from.

Usage: python3 evals/seed/answers.py   (after evals/seed/server.sh up; rewrites the references)
"""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

SEED_DIR = Path(__file__).resolve().parent
TASKS_DIR = SEED_DIR.parent / "tasks"

FLAKY_JOB, TRIGGER_JOB, COMPOSITE_JOB = "Shop_Backend_IntegrationTests", "Shop_Backend_Build", "Shop_Release_Verify"
UNIT_JOB, FRONTEND_JOB = "Shop_Backend_UnitTests", "Shop_Frontend_Build"
HISTORY_TEST = "com.acme.shop.it.CheckoutFlowIT.completesOrderWithSavedCard"
TRIGGER_KINDS = {"vcs": "vcs", "schedule": "schedule", "snapshotDependency": "dependency", "user": "user"}
BUILD_FIELDS = "build(id,number,buildTypeId,status,branchName,failedToStart,triggered(type,user(username)))"


class Server:
    def __init__(self, tc):
        self.tc = tc

    def builds(self, locator: str, count: int = 1000) -> list[dict]:
        """Finished runs, newest first, on every branch unless the locator names one."""
        branch = "" if "branch:" in locator else ",branch:default:any"
        return self.tc.get(f"/app/rest/builds?locator={locator}{branch},state:finished,count:{count}"
                           f"&fields={BUILD_FIELDS}")["build"]

    def failed_tests(self, build_id: int) -> list[str]:
        found = self.tc.get(f"/app/rest/testOccurrences?locator=build:(id:{build_id}),status:FAILURE,count:1000"
                            "&fields=testOccurrence(name)")
        return sorted(t["name"] for t in found.get("testOccurrence", []))

    def changes(self, build_id: int) -> list[dict]:
        return self.tc.get(f"/app/rest/changes?locator=build:(id:{build_id})&fields=change(version,username,comment)")["change"]

    def jobs(self, project: str, recursive: bool = True) -> list[dict]:
        scope = "affectedProject" if recursive else "project"
        return self.tc.get(f"/app/rest/buildTypes?locator={scope}:(id:{project})&fields=buildType(id,projectId,"
                           "steps(step(properties(property(name,value)))))")["buildType"]


def run_ref(build: dict) -> dict:
    return {"job": build["buildTypeId"], "number": build["number"], "branch": build.get("branchName", "")}


def facts(s: Server) -> dict:
    jobs = s.jobs("_Root")
    # per run: number, branch, status, trigger, user, failed tests, commits
    runs = {j["id"]: [[b["number"], b.get("branchName", ""), "FAILED_TO_START" if b.get("failedToStart") else b["status"],
                       b["triggered"]["type"], b["triggered"].get("user", {}).get("username", ""),
                       s.failed_tests(b["id"]), [f"{c['version'][:12]} {c['username']}" for c in s.changes(b["id"])]]
                      for b in s.builds(f"buildType:(id:{j['id']}),failedToStart:any")] for j in jobs}
    queue = s.tc.get("/app/rest/buildQueue?fields=build(buildTypeId,branchName,waitReason,"
                     "triggered(user(username)),compatibleAgents(count))")["build"]
    pools = s.tc.get("/app/rest/agentPools?fields=agentPool(name,projects(project(id)),"
                     "agents(agent(name,enabled,authorized)))")["agentPool"]
    agents = s.tc.get("/app/rest/agents?locator=authorized:any&fields=agent(name,enabled,authorized,"
                      "pool(name),enabledInfo(comment(text)))")["agent"]
    return {
        "projects": s.tc.get("/app/rest/projects?fields=project(id,name,parentProjectId)")["project"],
        "jobs": {j["id"]: sorted({p["value"] for st in j["steps"].get("step", []) for p in st["properties"]["property"]
                                  if p["name"] == "plugin.docker.imageId"}) for j in jobs},
        "runs": runs,
        "queue": [{"job": q["buildTypeId"], "branch": q.get("branchName", ""), "user": q["triggered"]["user"]["username"],
                   "compatible_agents": q["compatibleAgents"]["count"]} for q in queue],
        "pools": [{"name": p["name"], "projects": sorted(x["id"] for x in p["projects"].get("project", [])),
                   "agents": sorted(a["name"] for a in p["agents"].get("agent", []))} for p in pools],
        "agents": [{"name": a["name"], "pool": a["pool"]["name"], "enabled": a["enabled"], "authorized": a["authorized"],
                    "comment": a.get("enabledInfo", {}).get("comment", {}).get("text", "")} for a in agents],
    }


def answer(fields: dict, **options) -> dict:
    return {"file": "answer.json", "answer": fields} | options


def task_answers(s: Server, f: dict) -> dict[str, dict]:
    tasks: dict[str, dict] = {}

    window = s.builds(f"buildType:(id:{FLAKY_JOB})", 20)
    failures: dict[str, int] = {}
    for b in window:
        for test in s.failed_tests(b["id"]):
            failures[test] = failures.get(test, 0) + 1
    tasks["flaky-tests"] = answer({"flaky": [{"test": t, "failed_runs": n} for t, n in sorted(failures.items())
                                             if 1 < n < len(window)]})

    history = [b for b in reversed(s.builds(f"buildType:(id:{FLAKY_JOB})")) if HISTORY_TEST in s.failed_tests(b["id"])]
    tasks["test-history"] = answer({"failed_runs": len(history), "first_failed_run": history[0]["number"],
                                    "branches": sorted({b["branchName"] for b in history})})

    shop_queue = [q for q in f["queue"] if q["job"].startswith("Shop_")]
    oldest = shop_queue[0]
    requirement = s.tc.get(f"/app/rest/buildTypes/id:{oldest['job']}/agent-requirements")["agent-requirement"][0]
    tasks["why-queued"] = answer({
        "job": oldest["job"], "triggered_by": oldest["user"],
        "missing_agent_parameter": next(p["value"] for p in requirement["properties"]["property"] if p["name"] == "property-name"),
        "compatible_agents": oldest["compatible_agents"], "queued_runs_in_project": len(shop_queue)})

    enabled = {a["name"] for a in f["agents"] if a["enabled"]}
    tasks["pool-capacity"] = answer({"pools": [
        {"name": p["name"], "agents": len(p["agents"]), "enabled_agents": len(enabled & set(p["agents"]))}
        for p in f["pools"] if "Shop_Release" in p["projects"]]})
    tasks["disabled-agents"] = answer({"disabled_agents": [
        {"name": a["name"], "pool": a["pool"]} for a in f["agents"] if not a["enabled"]]})

    shop_jobs = {j["id"] for j in s.jobs("Shop")}
    images: dict[str, int] = {}
    for job in sorted(shop_jobs):
        for image in f["jobs"][job]:
            images[image] = images.get(image, 0) + 1
    tasks["settings-search"] = answer({"jobs_with_docker_steps": sum(1 for j in shop_jobs if f["jobs"][j]), "images": images})

    triggers = dict.fromkeys(TRIGGER_KINDS.values(), 0)
    for b in s.builds(f"buildType:(id:{TRIGGER_JOB})", 30):
        triggers[TRIGGER_KINDS[b["triggered"]["type"]]] += 1
    tasks["trigger-history"] = answer(triggers)

    failed_chains = []
    for b in s.builds(f"buildType:(id:{COMPOSITE_JOB}),status:FAILURE"):
        chain = s.builds(f"snapshotDependency:(to:(id:{b['id']}),includeInitial:false),defaultFilter:false")
        failed = [d for d in chain if d["status"] == "FAILURE" and not d.get("failedToStart")]
        roots = [d for d in failed if not any(
            x["id"] in {y["id"] for y in failed} for x in
            s.builds(f"snapshotDependency:(to:(id:{d['id']}),includeInitial:false),defaultFilter:false"))]
        failed_chains.append({"number": b["number"], "branch": b["branchName"],
                              "root_cause_jobs": sorted(d["buildTypeId"] for d in roots),
                              "failed_tests": sorted({t for d in roots for t in s.failed_tests(d["id"])})})
    tasks["composite-failure"] = answer({"failed_runs": failed_chains})

    tasks["find-builds"] = answer({"runs": [run_ref(b) for b in s.builds(
        "affectedProject:(id:Shop_Backend),status:FAILURE", 10)]}, ordered=["runs"])

    on_main = s.builds(f"buildType:(id:{UNIT_JOB}),branch:(name:main)")
    first_failing = on_main[0]
    for b in on_main:  # newest first: walk back while the job keeps failing
        if b["status"] != "FAILURE":
            break
        first_failing = b
    culprit = s.changes(first_failing["id"])
    assert on_main[0]["status"] == "FAILURE" and len(culprit) == 1, "unit tests: expected one culprit commit"
    tasks["investigate-failure"] = answer({
        "failed_tests": s.failed_tests(on_main[0]["id"]), "first_failing_run": first_failing["number"],
        "culprit_commit": culprit[0]["version"], "culprit_author": culprit[0]["username"]}, prefix=["culprit_commit"])

    last5 = s.builds(f"buildType:(id:{TRIGGER_JOB}),branch:(name:main)", 5)
    broken = [b for b in s.builds(f"buildType:(id:{TRIGGER_JOB}),branch:(name:main),status:FAILURE")]
    breaking = s.changes(broken[-1]["id"])
    assert len(breaking) == 1 and any(b["status"] == "FAILURE" for b in last5), "build: expected one breaking commit"
    tasks["daily-loop"] = answer({
        "runs": [{"number": b["number"], "status": b["status"]} for b in last5],
        "breaking_commit": breaking[0]["version"], "breaking_commit_author": breaking[0]["username"]},
        ordered=["runs"], prefix=["breaking_commit"])

    last10 = s.builds(f"buildType:(id:{UNIT_JOB})", 10)
    tasks["inspect-url"] = answer({
        "job": UNIT_JOB, "succeeded": sum(b["status"] == "SUCCESS" for b in last10),
        "failed": sum(b["status"] == "FAILURE" for b in last10), "latest_status": last10[0]["status"]})

    latest: dict[str, str] = {}
    for b in s.builds(f"buildType:(id:{FRONTEND_JOB})"):
        latest.setdefault(b["branchName"], b["status"])
    tasks["branch-status"] = answer({"branches": latest})

    health = []
    for project in sorted(p["id"] for p in f["projects"] if p.get("parentProjectId") == "Shop"):
        jobs = [j["id"] for j in s.jobs(project, recursive=False)]
        default = {j: [r for r in f["runs"][j] if r[1] in ("main", "") and r[2] != "FAILED_TO_START"] for j in jobs}
        health.append({"project": project, "jobs": len(jobs), "never_run": sum(not f["runs"][j] for j in jobs),
                       "failing": sum(bool(runs) and runs[0][2] == "FAILURE" for runs in default.values())})
    tasks["cross-project"] = answer({"subprojects": health})

    tasks["explore-infrastructure"] = answer({
        "top_level_projects": sorted(p["id"] for p in f["projects"] if p.get("parentProjectId") == "_Root"),
        "shop_subprojects": sorted(p["id"] for p in f["projects"] if p.get("parentProjectId") == "Shop"),
        "agent_pools": len(f["pools"]), "agents": len(f["agents"])})

    backend50 = s.builds("affectedProject:(id:Shop_Backend)", 50)
    unit_latest = s.tc.get(f"/app/rest/testOccurrences?locator=build:(id:{on_main[0]['id']}),count:1000"
                           "&fields=testOccurrence(name,duration)")["testOccurrence"]
    slowest = max(unit_latest, key=lambda t: t.get("duration", 0))
    tasks["hallucination-resistance"] = answer({
        "failed_runs": sum(b["status"] == "FAILURE" for b in backend50), "error_file": "backend/src/Cart.java",
        "slowest_test": slowest["name"], "slowest_test_ms": slowest["duration"]})

    tasks["locate-and-link"] = {"file": "teamcity.toml", "answer": {
        "server": [{"url": "http://teamcity:8111", "project": "Shop_Backend"}]}}
    linked = s.builds("affectedProject:(id:Shop_Frontend)", 1)[0]
    tasks["use-repository-link"] = answer({"job": linked["buildTypeId"], "number": linked["number"],
                                           "branch": linked["branchName"], "status": linked["status"]})
    tasks["negative-unrelated"] = {
        "file": "total_revenue.py", "run": ["python3", "total_revenue.py"], "stdout": "987.25",
        "files": {"sales.csv": "order_id,customer,amount\n1001,Lena,120.50\n1002,Omar,300.00\n1003,Ines,566.75\n"}}
    return tasks


def write_ground_truth(tc) -> str:
    server = Server(tc)
    found = facts(server)
    tasks = task_answers(server, found)
    fingerprint = hashlib.sha256(json.dumps(found, sort_keys=True).encode()).hexdigest()[:12]
    manifest = {"fingerprint": fingerprint, "facts": found, "tasks": tasks}
    (SEED_DIR / "manifest.json").write_text(json.dumps(manifest, indent=1) + "\n")
    for task, spec in tasks.items():
        (TASKS_DIR / task / "tests").mkdir(parents=True, exist_ok=True)
        (TASKS_DIR / task / "tests" / "expected.json").write_text(json.dumps(spec, indent=1) + "\n")
    return fingerprint


if __name__ == "__main__":
    import re
    import sys

    sys.path.insert(0, str(SEED_DIR))
    from seed import TC, sh

    name = sys.argv[1] if len(sys.argv) > 1 else "tc-eval-server"
    token = re.findall(r"Super user authentication token: (\d+)", sh("docker", "logs", name))[-1]
    print(write_ground_truth(TC("", token)))
