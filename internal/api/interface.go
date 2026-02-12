package api

import (
	"context"
	"io"
)

// ClientInterface defines the TeamCity API client interface.
// Cmd package uses this interface for dependency injection in tests.
type ClientInterface interface {
	// Server
	GetServer() (*Server, error)
	ServerVersion() (*Server, error)
	CheckVersion() error
	SupportsFeature(feature string) bool

	// Users
	GetCurrentUser() (*User, error)
	GetUser(username string) (*User, error)
	UserExists(username string) bool
	CreateUser(req CreateUserRequest) (*User, error)
	CreateAPIToken(name string) (*Token, error)
	DeleteAPIToken(name string) error

	// Projects
	GetProjects(opts ProjectsOptions) (*ProjectList, error)
	GetProject(id string) (*Project, error)
	CreateProject(req CreateProjectRequest) (*Project, error)
	ProjectExists(id string) bool
	CreateSecureToken(projectID, value string) (string, error)
	GetSecureValue(projectID, token string) (string, error)
	GetVersionedSettingsStatus(projectID string) (*VersionedSettingsStatus, error)
	GetVersionedSettingsConfig(projectID string) (*VersionedSettingsConfig, error)
	ExportProjectSettings(projectID, format string, useRelativeIds bool) ([]byte, error)

	// Build Types (Jobs)
	GetBuildTypes(opts BuildTypesOptions) (*BuildTypeList, error)
	GetBuildType(id string) (*BuildType, error)
	SetBuildTypePaused(id string, paused bool) error
	CreateBuildType(projectID string, req CreateBuildTypeRequest) (*BuildType, error)
	BuildTypeExists(id string) bool
	CreateBuildStep(buildTypeID string, step BuildStep) error
	GetVcsRootEntries(buildTypeID string) (*VcsRootEntries, error)
	SetBuildTypeSetting(buildTypeID, setting, value string) error

	// Builds (Runs)
	GetBuilds(opts BuildsOptions) (*BuildList, error)
	GetBuild(ref string) (*Build, error)
	ResolveBuildID(ref string) (string, error)
	RunBuild(buildTypeID string, opts RunBuildOptions) (*Build, error)
	CancelBuild(buildID string, comment string) error
	GetBuildLog(buildID string) (string, error)
	PinBuild(buildID string, comment string) error
	UnpinBuild(buildID string) error
	AddBuildTags(buildID string, tags []string) error
	GetBuildTags(buildID string) (*TagList, error)
	RemoveBuildTag(buildID string, tag string) error
	SetBuildComment(buildID string, comment string) error
	GetBuildComment(buildID string) (string, error)
	DeleteBuildComment(buildID string) error
	GetBuildChanges(buildID string) (*ChangeList, error)
	GetBuildTests(buildID string, failedOnly bool, limit int) (*TestOccurrences, error)
	GetBuildProblems(buildID string) (*ProblemOccurrences, error)
	UploadDiffChanges(patch []byte, description string) (string, error)

	// Artifacts
	GetArtifacts(buildID string) (*Artifacts, error)
	DownloadArtifact(buildID, artifactPath string) ([]byte, error)
	DownloadArtifactTo(ctx context.Context, buildID, artifactPath string, w io.Writer) (int64, error)

	// Build Queue
	GetBuildQueue(opts QueueOptions) (*BuildQueue, error)
	RemoveFromQueue(id string) error
	SetQueuedBuildPosition(buildID string, position int) error
	MoveQueuedBuildToTop(buildID string) error
	ApproveQueuedBuild(buildID string) error
	GetQueuedBuildApprovalInfo(buildID string) (*ApprovalInfo, error)

	// Parameters
	GetProjectParameters(projectID string) (*ParameterList, error)
	GetProjectParameter(projectID, name string) (*Parameter, error)
	SetProjectParameter(projectID, name, value string, secure bool) error
	DeleteProjectParameter(projectID, name string) error
	GetBuildTypeParameters(buildTypeID string) (*ParameterList, error)
	GetBuildTypeParameter(buildTypeID, name string) (*Parameter, error)
	SetBuildTypeParameter(buildTypeID, name, value string, secure bool) error
	DeleteBuildTypeParameter(buildTypeID, name string) error
	GetParameterValue(path string) (string, error)

	// Agents
	GetAgents(opts AgentsOptions) (*AgentList, error)
	GetAgent(id int) (*Agent, error)
	GetAgentByName(name string) (*Agent, error)
	AuthorizeAgent(id int, authorized bool) error
	EnableAgent(id int, enabled bool) error
	RebootAgent(ctx context.Context, id int, afterBuild bool) error
	GetAgentCompatibleBuildTypes(id int) (*BuildTypeList, error)
	GetAgentIncompatibleBuildTypes(id int) (*CompatibilityList, error)

	// Agent Pools
	GetAgentPools(fields []string) (*PoolList, error)
	GetAgentPool(id int) (*Pool, error)
	AddProjectToPool(poolID int, projectID string) error
	RemoveProjectFromPool(poolID int, projectID string) error
	SetAgentPool(agentID int, poolID int) error

	// Raw API access
	RawRequest(method, path string, body io.Reader, headers map[string]string) (*RawResponse, error)
}

// Verify *Client implements ClientInterface at compile time
var _ ClientInterface = (*Client)(nil)
