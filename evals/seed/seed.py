#!/usr/bin/env python3
"""Build the seeded TeamCity server the evals query, and snapshot it as a Docker image.

Starts a clean server and three agents, creates the fixture (projects, jobs, pools, a git
repository inside the server container), runs a fixed history of builds, leaves a few runs
queued that can never start, then stops everything and commits the server container as
`teamcity-cli-eval-server:seeded`. Ground truth read back from the server goes to manifest.json
and to each task's tests/expected.json (see answers.py).

Usage: python3 evals/seed/seed.py [--keep]   (--keep leaves the seed containers running)
"""

from __future__ import annotations

import base64
import json
import re
import secrets
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

SEED_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SEED_DIR))

VERSION = "2026.2.2"  # pinned: the snapshot holds this version's data directory
SERVER_IMAGE = f"jetbrains/teamcity-server:{VERSION}"
AGENT_IMAGE = f"jetbrains/teamcity-agent:{VERSION}"
SNAPSHOT = "teamcity-cli-eval-server:seeded"
NETWORK = "tc-eval"
# The seed agents get a network with no route out. On a cloud build agent they would otherwise read
# the host's instance metadata and register with the host's own TeamCity server.
AGENT_NETWORK = "tc-eval-agents"
SERVER = "tc-eval-seed"
HOST_URL = "http://127.0.0.1:18111"
ROOT_URL = "http://teamcity:8111"  # the server's address on the tc-eval network
DATA = "/opt/teamcity/seed-data"  # outside the image's volumes, so `docker commit` keeps it
REPO = "/opt/teamcity/seed-repos/shop"
SERVER_OPTS = (
    "-Dteamcity.installation.completed=true -Dteamcity.startup.maintenance=false "
    "-Dteamcity.licenseAgreement.accepted=true -Dteamcity.tools.bundled.installOnStartup=false "
    "-Dteamcity.agent.inactive.threshold.secs=600 -Dteamcity.git.allowFileUrl=true"
)

# agent name -> pool; the last builder is disabled at the end
AGENTS = {"linux-default-01": "Default", "linux-builder-01": "Linux Builders", "linux-builder-02": "Linux Builders"}
DISABLED_AGENT = ("linux-builder-02", "Disk replacement scheduled, see INFRA-212")
POOLS = {  # pool -> projects that may use it
    "Linux Builders": ["Shop", "Shop_Backend", "Shop_Frontend", "Shop_Release"],
    "macOS Signing": ["Shop_Release"],
    "Database Hosts": ["Infra"],
}
PROJECTS = [  # (id, name, parent)
    ("Shop", "Acme Shop", "_Root"),
    ("Shop_Backend", "Backend", "Shop"),
    ("Shop_Frontend", "Frontend", "Shop"),
    ("Shop_Release", "Release", "Shop"),
    ("Infra", "Platform Infrastructure", "_Root"),
]
DEVELOPERS = ["alice", "bob", "carol"]


def sh(*args: str, stdin: str | None = None, check: bool = True) -> str:
    out = subprocess.run(args, input=stdin, capture_output=True, text=True)
    if check and out.returncode:
        raise RuntimeError(f"{' '.join(args[:6])}: {out.stderr.strip() or out.stdout.strip()}")
    return out.stdout.strip()


class TC:
    """Minimal TeamCity REST client (basic auth)."""

    def __init__(self, user: str, password: str):
        self.auth = "Basic " + base64.b64encode(f"{user}:{password}".encode()).decode()

    def call(self, method: str, path: str, body=None, text: bool = False):
        data = None if body is None else (body if text else json.dumps(body)).encode()
        req = urllib.request.Request(HOST_URL + path, data=data, method=method, headers={
            "Authorization": self.auth, "Accept": "text/plain" if text else "application/json", "Origin": HOST_URL,
            "Content-Type": "text/plain" if text else "application/json"})
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                raw = resp.read().decode()
        except urllib.error.HTTPError as e:
            raise RuntimeError(f"{method} {path}: HTTP {e.code} {e.read().decode()[:400]}") from None
        try:
            return json.loads(raw) if raw else None
        except json.JSONDecodeError:
            return raw

    def get(self, path: str):
        return self.call("GET", path)

    def post(self, path: str, body=None):
        return self.call("POST", path, body)


def props(**kv: str) -> dict:
    return {"property": [{"name": k.replace("__", "."), "value": v} for k, v in kv.items()]}


def wait_for(what: str, probe, timeout: float = 300, interval: float = 1.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            if (value := probe()):
                return value
        except (RuntimeError, OSError):
            pass
        time.sleep(interval)
    raise TimeoutError(f"timed out waiting for {what}")


# --- containers ---------------------------------------------------------------------------

def start_server() -> TC:
    sh("docker", "rm", "-f", "-v", SERVER, "tc-eval-server", *AGENTS, check=False)
    if not sh("docker", "network", "ls", "-q", "--filter", f"name=^{NETWORK}$"):
        sh("docker", "network", "create", NETWORK)
    sh("docker", "run", "-d", "--name", SERVER, "--network", NETWORK, "--network-alias", "teamcity",
       "-p", "127.0.0.1:18111:8111", "-e", f"TEAMCITY_DATA_PATH={DATA}",
       "-e", "TEAMCITY_SERVER_MEM_OPTS=-Xmx1500m -XX:ReservedCodeCacheSize=400m",
       "-e", f"TEAMCITY_SERVER_OPTS={SERVER_OPTS}", "-e", "TEAMCITY_STOP_WAIT_TIME=120",
       "--entrypoint", "sh", SERVER_IMAGE, "-c", f"mkdir -p {DATA} && exec /run-services.sh")
    sh("docker", "network", "rm", AGENT_NETWORK, check=False)
    sh("docker", "network", "create", "--internal", AGENT_NETWORK)
    sh("docker", "network", "connect", "--alias", "teamcity", AGENT_NETWORK, SERVER)
    token = wait_for("superuser token", lambda: re.findall(
        r"Super user authentication token: (\d+)", sh("docker", "logs", SERVER, check=False)))[-1]
    tc = TC("", token)
    wait_for("server", lambda: tc.get("/app/rest/server"), timeout=600, interval=3)
    urllib.request.urlopen(urllib.request.Request(
        f"{HOST_URL}/showAgreement.html?agree=true&super={token}", method="POST"), timeout=30).close()
    tc.call("PUT", "/app/rest/server/globalSettings", tc.get("/app/rest/server/globalSettings") | {"rootUrl": ROOT_URL})
    return tc


def start_agents(tc: TC) -> dict[str, int]:
    for name in AGENTS:
        sh("docker", "run", "-d", "--name", name, "--network", AGENT_NETWORK, "-e", f"SERVER_URL={ROOT_URL}",
           "-e", f"AGENT_NAME={name}", AGENT_IMAGE)

    def ready():
        agents = tc.get("/app/rest/agents?locator=authorized:any&fields=agent(id,name,authorized,connected,uptodate)")["agent"]
        for a in agents:
            if not a["authorized"]:
                tc.call("PUT", f"/app/rest/agents/id:{a['id']}/authorized", "true", text=True)
        ok = {a["name"]: a["id"] for a in agents if a["authorized"] and a["connected"] and a["uptodate"]}
        return ok if set(ok) == set(AGENTS) else None

    try:
        return wait_for("agents to connect and upgrade", ready, timeout=420, interval=5)
    except TimeoutError:  # say why before failing: this is the step that depends on the host most
        print(tc.get("/app/rest/agents?locator=authorized:any&fields=agent(name,authorized,connected,uptodate)"), file=sys.stderr)
        for name in AGENTS:
            print(f"--- {name}\n" + sh("sh", "-c", f"docker logs --tail 25 {name} 2>&1", check=False), file=sys.stderr)
        raise


# --- fixture: users, projects, pools, jobs ---------------------------------------------------

def create_users(tc: TC) -> dict[str, TC]:
    settings = tc.get("/app/rest/server/authSettings")
    settings["perProjectPermissions"] = True
    tc.call("PUT", "/app/rest/server/authSettings", settings)
    for pid, name, parent in PROJECTS:  # roles are scoped to projects, so these come first
        tc.post("/app/rest/projects", {"id": pid, "name": name, "parentProject": {"locator": f"id:{parent}"}})
    users = {}
    for name, role, scope in [(d, "PROJECT_DEVELOPER", "p:Shop") for d in DEVELOPERS] + \
                             [("dave", "PROJECT_DEVELOPER", "p:Infra"), ("eval-viewer", "PROJECT_VIEWER", "p:_Root")]:
        password = secrets.token_hex(16)
        tc.post("/app/rest/users", {"username": name, "name": name.capitalize(), "password": password,
                                    "email": f"{name}@acme.example",
                                    "roles": {"role": [{"roleId": role, "scope": scope}]}})
        users[name] = TC(name, password)
    return users


def script_step(name: str, script: str, image: str = "") -> dict:
    p = {"script__content": script, "use__custom__script": "true"}
    if image:
        p["plugin__docker__imageId"] = image
        p["plugin__docker__imagePlatform"] = "linux"
    return {"name": name, "type": "simpleRunner", "properties": props(**p)}


def snapshot_dep(job: str, on_failure: str) -> dict:
    return {"type": "snapshot_dependency", "source-buildType": {"id": job}, "properties": props(
        **{"run-build-if-dependency-failed": on_failure, "run-build-if-dependency-failed-to-start": on_failure,
           "run-build-on-the-same-agent": "false", "take-started-build-with-same-revisions": "true",
           "take-successful-builds-only": "true"})}


def cron_trigger(hour: str, minute: str = "0", second: str = "0", pending_only: bool = True) -> dict:
    return {"type": "schedulingTrigger", "properties": props(
        schedulingPolicy="cron", cronExpression_sec=second, cronExpression_min=minute, cronExpression_hour=hour,
        cronExpression_dm="*", cronExpression_month="*", cronExpression_dw="?", timezone="SERVER",
        triggerBuildWithPendingChangesOnly=str(pending_only).lower(), branchFilter="+:<default>")}


VCS_TRIGGER = {"type": "vcsTrigger", "properties": props(quietPeriodMode="DO_NOT_USE", branchFilter="+:*")}

# id -> (project, name, steps, extras). Jobs with Docker steps never run: the agents have no Docker.
JOBS: dict[str, tuple[str, str, list[dict], dict]] = {
    "Shop_Backend_Build": ("Shop_Backend", "Build", [script_step("Compile", "sh ci/backend-build.sh")],
                           {"triggers": [VCS_TRIGGER]}),
    "Shop_Backend_UnitTests": ("Shop_Backend", "Unit Tests", [script_step("Run unit tests", "sh ci/backend-unit.sh")], {}),
    "Shop_Backend_IntegrationTests": ("Shop_Backend", "Integration Tests", [
        script_step("Run integration tests", "sh ci/backend-integration.sh %build.counter%")], {}),
    "Shop_Backend_ApiDocs": ("Shop_Backend", "API Docs", [
        script_step("Lint OpenAPI spec", "redocly lint backend/openapi.yaml", "redocly/cli:1.25.11"),
        script_step("Publish", "sh ci/publish-docs.sh")], {}),
    "Shop_Backend_DbMigrations": ("Shop_Backend", "DB Migrations", [
        script_step("Validate migrations", "flyway validate", "flyway/flyway:10.22")], {}),
    "Shop_Frontend_Build": ("Shop_Frontend", "Build", [script_step("Bundle", "sh ci/frontend-build.sh")], {}),
    "Shop_Frontend_Lint": ("Shop_Frontend", "Lint", [script_step("ESLint", "npx eslint frontend/src", "node:22-alpine")], {}),
    "Shop_Frontend_E2E": ("Shop_Frontend", "E2E Tests", [
        script_step("Install dependencies", "npm ci", "node:22-alpine"),
        script_step("Playwright", "npx playwright test", "mcr.microsoft.com/playwright:v1.49.1-jammy")], {}),
    "Shop_Frontend_Storybook": ("Shop_Frontend", "Storybook", [
        script_step("Build storybook", "npm run build-storybook", "node:22-alpine")], {}),
    "Shop_Release_Package": ("Shop_Release", "Package", [script_step("Assemble", "echo Assembling shop-%build.number%.tar.gz")],
                             {"deps": ["Shop_Backend_Build", "Shop_Frontend_Build"]}),
    "Shop_Release_Verify": ("Shop_Release", "Release Verification", [],
                            {"deps": ["Shop_Release_Package", "Shop_Backend_UnitTests"], "composite": True}),
    "Shop_Release_Sbom": ("Shop_Release", "SBOM", [
        script_step("Collect npm licenses", "npx license-checker --json", "node:22-alpine"),
        script_step("Generate SBOM", "syft dir:. -o cyclonedx-json", "anchore/syft:v1.18.1")], {}),
    "Shop_Release_Deploy": ("Shop_Release", "Deploy to Staging", [script_step("Deploy", "sh ci/deploy.sh staging")],
                            {"requirements": [("exists", "env.STAGING_KUBECONFIG")]}),
    "Infra_BaseImages": ("Infra", "Base Images", [
        script_step("Build images", "docker buildx bake", "docker:27-cli")], {"no_vcs": True}),
    "Infra_NightlyCleanup": ("Infra", "Nightly Cleanup", [script_step("Prune", "echo Pruned 0 stale workspaces")],
                             {"triggers": [cron_trigger("2", "30")], "no_vcs": True}),
    "Infra_DbBackup": ("Infra", "Database Backup", [script_step("Dump", "sh /opt/backup/dump.sh")],
                       {"requirements": [("exists", "env.PGHOST")], "no_vcs": True}),
}


def create_fixture(tc: TC) -> None:
    tc.post("/app/rest/vcs-roots", {
        "id": "Shop_Repo", "name": "shop", "vcsName": "jetbrains.git", "project": {"id": "Shop"},
        "modificationCheckInterval": 5,
        "properties": props(url=REPO, branch="refs/heads/main", authMethod="ANONYMOUS", usernameStyle="USERID",
                            **{"teamcity:branchSpec": "+:refs/heads/*"})})
    for jid, (project, name, steps, extra) in JOBS.items():
        base = f"/app/rest/buildTypes/id:{jid}"
        tc.post(f"/app/rest/projects/id:{project}/buildTypes", {"id": jid, "name": name})
        settings = {"maximumNumberOfBuilds": "1", "checkoutMode": "ON_SERVER"}
        if extra.get("composite"):
            settings["buildConfigurationType"] = "COMPOSITE"
        for key, value in settings.items():
            tc.call("PUT", f"{base}/settings/{key}", value, text=True)
        if not extra.get("no_vcs"):
            tc.post(f"{base}/vcs-root-entries", {"id": "Shop_Repo", "vcs-root": {"id": "Shop_Repo"}})
        for step in steps:
            tc.post(f"{base}/steps", step)
        for trigger in extra.get("triggers", []):
            tc.post(f"{base}/triggers", trigger)
        for dep in extra.get("deps", []):
            tc.post(f"{base}/snapshot-dependencies",
                    snapshot_dep(dep, "RUN_ADD_PROBLEM" if extra.get("composite") else "MAKE_FAILED_TO_START"))
        for kind, param in extra.get("requirements", []):
            tc.post(f"{base}/agent-requirements", {"type": kind, "properties": props(**{"property-name": param})})
    for pool, projects in POOLS.items():
        pool_id = tc.post("/app/rest/agentPools", {"name": pool})["id"]
        for project in projects:
            tc.post(f"/app/rest/agentPools/id:{pool_id}/projects", {"id": project})
            if pool == "Linux Builders":  # Shop builds only there, not in Default
                tc.call("DELETE", f"/app/rest/agentPools/id:0/projects/id:{project}")


def assign_agents(tc: TC, agent_ids: dict[str, int]) -> None:
    pools = {p["name"]: p["id"] for p in tc.get("/app/rest/agentPools")["agentPool"]}
    for name, pool in AGENTS.items():
        tc.post(f"/app/rest/agentPools/id:{pools[pool]}/agents", {"id": agent_ids[name]})


# --- repository and history -----------------------------------------------------------------

INITIAL_FILES = {
    "README.md": "# Acme Shop\n",
    "backend/src/Cart.java": "package com.acme.shop.cart;\n\npublic class Cart {\n}\n",
    "backend/pricing.properties": "rounding=half_up\ncoupons=false\n",
    "frontend/src/cart.ts": "export interface CartItem {\n  sku: string;\n  quantity: number;\n}\n",
} | {f"ci/{p.name}": p.read_text() for p in sorted((SEED_DIR / "repo" / "ci").iterdir())}


class Repo:
    """The git repository inside the server container; fixed dates keep commit hashes stable."""

    def __init__(self):
        self.count = 0
        sh("docker", "exec", SERVER, "mkdir", "-p", REPO)
        self.run("git init -q -b main . && git config user.name seed && git config user.email seed@acme.example")

    def run(self, script: str) -> str:
        return sh("docker", "exec", "-i", "-w", REPO, SERVER, "sh", "-e", stdin=script)

    def commit(self, branch: str, author: str, message: str, files: dict[str, str], base: str = "") -> str:
        self.count += 1
        date = f"2026-11-{2 + self.count // 8:02d}T{9 + self.count % 8:02d}:00:00Z"
        lines = [f"if git rev-parse -q --verify HEAD >/dev/null; then git checkout -q {branch} 2>/dev/null "
                 f"|| git checkout -q -b {branch} {base}; fi"]
        for path, content in files.items():
            lines += [f"mkdir -p $(dirname {path})", f"cat > {path} <<'SEED_EOF'\n{content}SEED_EOF"]
        lines += ["git add -A",
                  f"GIT_AUTHOR_DATE={date} GIT_COMMITTER_DATE={date} GIT_COMMITTER_NAME={author} "
                  f"GIT_COMMITTER_EMAIL={author}@acme.example git commit -q --author='{author} <{author}@acme.example>' "
                  f"-m '{message}'", "git rev-parse HEAD"]
        return self.run("\n".join(lines))


class History:
    def __init__(self, tc: TC, users: dict[str, TC], repo: Repo):
        self.tc, self.users, self.repo = tc, users, repo

    def finished(self, job: str) -> int:
        return self.tc.get(f"/app/rest/builds?locator=buildType:{job},branch:default:any,state:finished,"
                           "failedToStart:any,canceled:any,count:1000&fields=count")["count"]

    def wait_idle(self) -> None:
        """Wait until nothing is running or startable; runs queued for good don't count."""
        def idle():
            running = self.tc.get("/app/rest/builds?locator=state:running,branch:default:any&fields=count")["count"]
            queued = self.tc.get("/app/rest/buildQueue?fields=build(buildTypeId)")["build"]
            return running == 0 and all(b["buildTypeId"] in STUCK_JOBS for b in queued)
        wait_for("builds to finish", idle, timeout=600, interval=0.5)

    def commit(self, branch: str, author: str, message: str, files: dict[str, str], base: str = "") -> None:
        """Commit and wait for the VCS trigger to run Shop_Backend_Build on it."""
        before = self.finished("Shop_Backend_Build")
        self.repo.commit(branch, author, message, files, base)
        if base:  # the VCS trigger ignores a branch until its first build
            return self.run("Shop_Backend_Build", branch, author)
        wait_for(f"VCS-triggered build for '{message}'", lambda: self.finished("Shop_Backend_Build") > before,
                 timeout=180, interval=0.5)
        self.wait_idle()

    def run(self, job: str, branch: str, user: str, rebuild: bool = False, wait: bool = True) -> None:
        body = {"buildType": {"id": job}} | ({"branchName": branch} if branch else {})
        if rebuild:
            body["triggeringOptions"] = {"rebuildAllDependencies": True}
        self.users[user].post("/app/rest/buildQueue", body)
        if wait:
            self.wait_idle()

    def scheduled(self, job: str, n: int) -> None:
        """Let a temporary every-15-seconds schedule start n runs."""
        before = self.finished(job)
        trigger = self.tc.post(f"/app/rest/buildTypes/id:{job}/triggers",
                               cron_trigger("*", "*", "0/15", pending_only=False))
        wait_for(f"{n} scheduled runs of {job}", lambda: self.finished(job) >= before + n, timeout=300, interval=0.5)
        self.tc.call("DELETE", f"/app/rest/buildTypes/id:{job}/triggers/{trigger['id']}")
        self.wait_idle()
        if (extra := self.finished(job) - before - n):
            raise RuntimeError(f"{job}: schedule started {extra} run(s) too many")


STUCK_JOBS = {"Infra_DbBackup", "Shop_Release_Deploy", "Shop_Frontend_E2E"}
FLAKY_JOB = "Shop_Backend_IntegrationTests"
MAIN, FEATURE, RELEASE = "main", "feature/cart-redesign", "release/2.4"
BROKEN_CART = "export interface CartItem {\n  sku: string;\n  quantity: string;\n  thumbnail: string;\n}\n"
FIXED_CART = BROKEN_CART.replace("quantity: string", "quantity: number")


def play_history(h: History) -> None:
    for job, user in [("Shop_Backend_Build", "alice"), ("Shop_Backend_UnitTests", "alice"),
                      ("Shop_Frontend_Build", "carol"), ("Infra_NightlyCleanup", "")]:
        h.run(job, MAIN if user else "", user or "dave")
    h.run("Infra_DbBackup", "", "dave", wait=False)  # never starts: no agent has env.PGHOST
    h.commit(MAIN, "bob", "Add coupon support to pricing", {"backend/pricing.properties": "rounding=half_up\ncoupons=true\n"})
    h.run("Shop_Backend_UnitTests", MAIN, "bob")
    h.run("Shop_Backend_Build", MAIN, "bob")
    h.commit(MAIN, "carol", "Cart: show item thumbnails", {"frontend/src/cart.ts": FIXED_CART})
    h.run("Shop_Frontend_Build", MAIN, "carol")
    h.commit(MAIN, "alice", "CI: cache the Gradle wrapper", {"ci/cache.txt": "gradle/wrapper\n"})
    h.commit(MAIN, "bob", "Pricing: extract tax calculator", {"backend/src/Tax.java": "package com.acme.shop.pricing;\n"})
    h.commit(MAIN, "carol", "Docs: describe the checkout flow", {"docs/checkout.md": "# Checkout\n"})
    h.run("Shop_Release_Verify", MAIN, "alice", rebuild=True)  # green
    h.run("Shop_Release_Deploy", MAIN, "carol", wait=False)  # never starts: no agent has env.STAGING_KUBECONFIG

    h.commit(RELEASE, "alice", "Bump version to 2.4.0", {"README.md": "# Acme Shop 2.4.0\n"}, base=MAIN)
    h.run("Shop_Backend_UnitTests", RELEASE, "alice")
    h.commit(FEATURE, "carol", "Cart redesign: new layout", {"frontend/src/layout.ts": "export const columns = 2;\n"}, base=MAIN)
    h.run("Shop_Frontend_Build", FEATURE, "carol")
    h.run("Shop_Frontend_E2E", FEATURE, "alice", wait=False)  # never starts: its steps need Docker

    users = ["alice", "bob", "carol", "bob", "alice"]
    branches = [MAIN, MAIN, FEATURE, MAIN, RELEASE, MAIN]
    for i in range(13):  # integration runs 1-13, one at a time: TeamCity merges identical queued runs
        h.run(FLAKY_JOB, branches[i % len(branches)], users[i % len(users)])
    h.scheduled("Shop_Backend_Build", 6)
    h.scheduled("Infra_NightlyCleanup", 3)

    h.commit(MAIN, "bob", "Simplify discount rounding", {"backend/pricing.properties": "rounding=floor\ncoupons=true\n"})
    h.run("Shop_Backend_UnitTests", MAIN, "bob")  # first failing run on main
    h.commit(FEATURE, "carol", "Cart redesign: quantity stepper", {"frontend/src/cart.ts": BROKEN_CART})
    h.run("Shop_Frontend_Build", FEATURE, "carol")  # fails
    h.commit(MAIN, "alice", "Upgrade logging library", {"backend/build.gradle": "logback=1.5.12\n"})
    h.run("Shop_Backend_UnitTests", MAIN, "alice")
    h.run("Shop_Backend_Build", MAIN, "carol")
    h.run("Shop_Release_Verify", MAIN, "alice", rebuild=True)  # fails: unit tests
    h.commit(MAIN, "bob", "Cart: use legacy price formatter", {
        "backend/src/Cart.java": "package com.acme.shop.cart;\n\nimport com.acme.shop.legacy.PriceFormatter;\n\npublic class Cart {\n}\n"})
    h.run("Shop_Backend_Build", MAIN, "bob")  # retry of the broken compile
    h.run("Shop_Backend_UnitTests", MAIN, "carol")
    h.commit(MAIN, "bob", "Revert legacy price formatter import", {"backend/src/Cart.java": INITIAL_FILES["backend/src/Cart.java"]})
    for user in ["alice", "carol", "alice"]:
        h.run("Shop_Backend_Build", MAIN, user)
    h.run("Shop_Release_Verify", FEATURE, "carol", rebuild=True)  # fails: frontend build, package cannot start

    for i in range(13, 26):  # integration runs 14-26
        h.run(FLAKY_JOB, branches[i % len(branches)], users[i % len(users)])
    h.commit(RELEASE, "alice", "Release notes for 2.4.0", {"CHANGELOG.md": "## 2.4.0\n- Coupons\n"})
    h.run("Shop_Release_Verify", RELEASE, "alice", rebuild=True)  # green: release has no rounding change
    h.run("Shop_Backend_UnitTests", MAIN, "bob")
    h.run("Shop_Backend_Build", RELEASE, "bob")
    h.run("Shop_Frontend_Build", MAIN, "alice")
    h.run("Shop_Release_Deploy", RELEASE, "alice", wait=False)
    h.wait_idle()
    if h.finished(FLAKY_JOB) != 26:  # backend-integration.sh keys its failures on run numbers 1-26
        raise RuntimeError(f"{FLAKY_JOB}: {h.finished(FLAKY_JOB)} runs, expected 26")


# --- snapshot -------------------------------------------------------------------------------

def finish(tc: TC, users: dict[str, TC], agent_ids: dict[str, int], keep: bool) -> None:
    from answers import write_ground_truth

    name, comment = DISABLED_AGENT
    tc.call("PUT", f"/app/rest/agents/id:{agent_ids[name]}/enabledInfo", {"status": False, "comment": {"text": comment}})
    token = users["eval-viewer"].post("/app/rest/users/current/tokens/eval")["value"]  # read-only: PROJECT_VIEWER
    sh("docker", "exec", "-i", SERVER, "sh", "-c", "cat > /opt/teamcity/eval-token", stdin=token)
    if keep:
        print(f"Seed server left running at {HOST_URL}; viewer token in {SERVER}:/opt/teamcity/eval-token")
    sh("docker", "rm", "-f", "-v", *AGENTS)
    sh("docker", "network", "disconnect", AGENT_NETWORK, SERVER, check=False)
    sh("docker", "network", "rm", AGENT_NETWORK, check=False)
    wait_for("agents to disconnect", lambda: not tc.get("/app/rest/agents?locator=connected:true")["count"], timeout=120)
    fingerprint = write_ground_truth(tc)
    if keep:
        return
    sh("docker", "stop", "-t", "150", SERVER)
    sh("docker", "commit", "--change", f"LABEL tc.eval.fingerprint={fingerprint}", SERVER, SNAPSHOT)
    sh("docker", "rm", "-f", "-v", SERVER)
    print(f"Snapshot {SNAPSHOT} (fixture {fingerprint})")


def main() -> None:
    started = time.time()
    tc = start_server()
    print(f"server up after {time.time() - started:.0f}s")
    users = create_users(tc)
    repo = Repo()
    repo.commit(MAIN, "alice", "Initial import", INITIAL_FILES)  # the VCS root's baseline: triggers nothing
    create_fixture(tc)
    agent_ids = start_agents(tc)
    assign_agents(tc, agent_ids)
    print(f"fixture and agents ready after {time.time() - started:.0f}s")
    play_history(History(tc, users, repo))
    print(f"history done after {time.time() - started:.0f}s")
    finish(tc, users, agent_ids, keep="--keep" in sys.argv)
    print(f"seeded in {time.time() - started:.0f}s")


if __name__ == "__main__":
    try:
        main()
    except BaseException:  # leave nothing running on the host after a failed seed
        if "--keep" not in sys.argv:
            sh("docker", "rm", "-f", "-v", SERVER, *AGENTS, check=False)
            sh("docker", "network", "rm", AGENT_NETWORK, check=False)
        raise
