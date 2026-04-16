type Node = Record<string, any> | undefined;

/**
 * Extract an identifier from a tree-item-ish node, checking the named keys
 * both directly and on a nested `data` field (for `{ kind, data }` shapes).
 */
export function nodeId(n: Node, ...keys: string[]): string | undefined {
  for (const k of keys) {
    const v = n?.[k] ?? n?.data?.[k];
    if (v != null) return String(v);
  }
  return undefined;
}

/**
 * Build the TeamCity web-UI path for a tree node. Pure: returns a path like
 * `/build/123` or `/pipeline/CI_Root`; the caller prepends the server origin.
 */
export function browserPathFor(node: Node): string | undefined {
  if (!node) return undefined;
  if (node.buildTypeId) return `/build/${node.id}`;
  if (node.kind === "pipeline") return `/pipeline/${node.data?.id ?? node.id}`;
  if (node.kind === "job") return `/pipeline/${node.pipelineId ?? node.data?.pipelineId}`;
  if (node.name && node.connected !== undefined) return `/agent/${node.id}`;
  if (node.id) return `/buildConfiguration/${node.id}`;
  return undefined;
}

/** Exponential backoff: base * 2^fails, capped. `fails=0` returns base. */
export function backoffDelay(baseMs: number, fails: number, maxMs = 120_000): number {
  if (fails <= 0) return baseMs;
  return Math.min(baseMs * 2 ** fails, maxMs);
}

/**
 * Pick the best authenticated server from a status list:
 * prefer the configured default, else the canonical cli.teamcity.com, else first authed.
 */
export function pickServer<T extends { server: string; status: string }>(
  servers: T[],
  defaultServer: string,
): T | undefined {
  const authed = servers.filter((s) => s.status === "authenticated");
  if (authed.length === 0) return undefined;
  return authed.find((s) => s.server === defaultServer)
    ?? authed.find((s) => s.server === "https://cli.teamcity.com")
    ?? authed[0];
}

/** Pure CLI-argument builders for each action. Testable without VS Code. */
export const argsFor = {
  login: (server: string) => ["auth", "login", "-s", server],
  loginGuest: (server: string) => ["auth", "login", "--guest", "-s", server],
  logout: () => ["auth", "logout"],
  authStatus: () => ["auth", "status", "--json"],
  runList: (statusFilter = "") => statusFilter
    ? ["run", "list", "--json", "--status", statusFilter]
    : ["run", "list", "--json"],
  runListBranch: (branch = "@this") => ["run", "list", "--branch", branch, "--limit", "1", "--json"],
  runListFavorites: () => ["run", "list", "--favorites", "--json"],
  queueList: () => ["queue", "list", "--json"],
  agentList: () => ["agent", "list", "--json"],
  pipelineList: () => ["pipeline", "list", "--json"],
  pipelineView: (id: string) => ["pipeline", "view", id, "--json"],
  pipelineCreate: () => ["pipeline", "create"],
  pipelineValidate: () => ["pipeline", "validate"],
  pipelinePush: () => ["pipeline", "push"],
  triggerRun: (jobId?: string) => jobId
    ? ["run", "start", jobId, "--watch"]
    : ["run", "start"],
  remoteRun: (jobId?: string) => jobId
    ? ["run", "start", jobId, "--local-changes"]
    : ["run", "start", "--local-changes"],
  viewLog: (id: string) => ["run", "log", id, "--raw"],
  viewFailedTests: (id: string) => ["run", "tests", id, "--failed", "--json"],
};
