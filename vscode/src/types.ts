export interface AuthServerStatus {
  server: string;
  auth_method: string;
  token_source?: string;
  user?: { id: number; username: string; name: string };
  server_info?: { version_major: number; version_minor: number; build_number: string };
  token_expiry?: string;
  status: string;
  error?: string;
  is_default?: boolean;
}

export type AuthStatus = AuthServerStatus[];

export interface Pipeline {
  id: string;
  name: string;
  parentProject?: { id: string; name: string };
  headBuildType?: { id: string };
  webUrl?: string;
  jobs?: { count: number; job: BuildType[] };
}

export interface PipelineList {
  count: number;
  pipeline: Pipeline[];
}

export interface BuildType {
  id: string;
  name: string;
  projectName?: string;
  projectId?: string;
  webUrl?: string;
  paused?: boolean;
}

export interface Build {
  id: number;
  buildTypeId: string;
  number?: string;
  status?: string;
  state?: string;
  personal?: boolean;
  branchName?: string;
  webUrl?: string;
  statusText?: string;
  queuedDate?: string;
  startDate?: string;
  finishDate?: string;
  buildType?: BuildType;
  percentageComplete?: number;
  agent?: { id: number; name: string };
  waitReason?: string;
}

export interface BuildList {
  count: number;
  build: Build[];
}

export interface QueuedBuild {
  id: number;
  buildTypeId: string;
  state?: string;
  branchName?: string;
  webUrl?: string;
  buildType?: BuildType;
  queuedDate?: string;
  waitReason?: string;
}

export interface BuildQueue {
  count: number;
  build: QueuedBuild[];
}

export interface Agent {
  id: number;
  name: string;
  connected?: boolean;
  enabled?: boolean;
  authorized?: boolean;
  webUrl?: string;
  pool?: { id: number; name: string };
  build?: Build;
}

export interface AgentList {
  count: number;
  agent: Agent[];
}
