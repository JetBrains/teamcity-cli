package analytics

const (
	EventInvoked  = "invoked"
	EventExecuted = "executed"

	EventLoginCompleted = "login.completed"
	EventLoginAbandoned = "login.abandoned"
	EventTokenLoaded    = "token.loaded"

	EventStarted       = "started"
	EventWatchFinished = "watch.finished"
	EventLogViewed     = "log.viewed"
	EventTestsViewed   = "tests.viewed"
	EventDiffViewed    = "diff.viewed"

	EventTerminalClosed = "terminal.closed"
	EventExecFinished   = "exec.finished"
	EventStateChanged   = "state.changed"

	EventValidated = "validated"
	EventCreated   = "created"
	EventSynced    = "synced"

	EventManaged = "managed"
)

const (
	SourceHuman     = "human"
	SourceAgent     = "agent"
	SourceCI        = "ci"
	SourceBuildStep = "build_step"
)

const (
	CINone          = "none"
	CIOther         = "other"
	CIGitHubActions = "github_actions"
	CIGitLab        = "gitlab"
	CIJenkins       = "jenkins"
	CICircleCI      = "circleci"
	CIBuildkite     = "buildkite"
	CIAzure         = "azure"
	CITravis        = "travis"
	CITeamCity      = "teamcity"
)

const (
	AuthSourceKeyring         = "keyring"
	AuthSourceEnv             = "env"
	AuthSourceBuildProperties = "build_properties"
	AuthSourceGuest           = "guest"
	AuthSourceNone            = "none"
)

const (
	AuthMethodToken = "token"
	AuthMethodGuest = "guest"
)

const (
	AuthStepServer = "server"
	AuthStepToken  = "token"
	AuthStepVerify = "verify"
)

const (
	ServerTypeCloud  = "cloud"
	ServerTypeOnPrem = "on_prem"
)

const (
	ErrorAuth       = "auth"
	ErrorPermission = "permission"
	ErrorNotFound   = "not_found"
	ErrorNetwork    = "network"
	ErrorValidation = "validation"
	ErrorReadOnly   = "read_only"
	ErrorInternal   = "internal"
	ErrorNone       = "none"
)

const (
	BuildStatusSuccess  = "success"
	BuildStatusFailure  = "failure"
	BuildStatusError    = "error"
	BuildStatusCanceled = "canceled"
)

const (
	LogModeFull   = "full"
	LogModeFailed = "failed"
	LogModeRaw    = "raw"
	LogModeFollow = "follow"
)

const (
	TestsFilterAll    = "all"
	TestsFilterFailed = "failed"
)

const (
	AgentActionEnable      = "enable"
	AgentActionDisable     = "disable"
	AgentActionAuthorize   = "authorize"
	AgentActionDeauthorize = "deauthorize"
	AgentActionMove        = "move"
	AgentActionReboot      = "reboot"
)

const (
	AgentExitUser         = "user"
	AgentExitTimeout      = "timeout"
	AgentExitDisconnected = "disconnected"
	AgentExitError        = "error"
)

const (
	PipelineActionPush = "push"
	PipelineActionPull = "pull"
)

const (
	SkillActionInstall = "install"
	SkillActionUpdate  = "update"
	SkillActionRemove  = "remove"
)

const (
	SkillScopeGlobal  = "global"
	SkillScopeProject = "project"
)
