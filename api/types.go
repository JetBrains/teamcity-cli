package api

import (
	"encoding/xml"
	"strings"
	"time"
)

// User represents a TeamCity user
type User struct {
	ID       int    `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Href     string `json:"href,omitempty"`
}

// Project represents a TeamCity project
type Project struct {
	ID              string `json:"id"`
	Name            string `json:"name,omitempty"`
	Description     string `json:"description,omitempty"`
	ParentProjectID string `json:"parentProjectId,omitempty"`
	Href            string `json:"href,omitempty"`
	WebURL          string `json:"webUrl,omitempty"`
}

// ProjectList represents a list of projects
type ProjectList struct {
	Count    int       `json:"count"`
	Projects []Project `json:"project"`
}

// BuildType represents a build configuration
type BuildType struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	ProjectName string   `json:"projectName,omitempty"`
	ProjectID   string   `json:"projectId,omitempty"`
	Href        string   `json:"href,omitempty"`
	WebURL      string   `json:"webUrl,omitempty"`
	Paused      bool     `json:"paused,omitempty"`
	Project     *Project `json:"project,omitempty"`
}

// BuildTypeList represents a list of build configurations
type BuildTypeList struct {
	Count      int         `json:"count"`
	BuildTypes []BuildType `json:"buildType"`
}

// Build represents a TeamCity build
type Build struct {
	ID                 int         `json:"id"`
	BuildTypeID        string      `json:"buildTypeId,omitempty"`
	Number             string      `json:"number,omitempty"`
	Status             string      `json:"status,omitempty"`
	State              string      `json:"state,omitempty"`
	Personal           bool        `json:"personal,omitempty"`
	BranchName         string      `json:"branchName,omitempty"`
	DefaultBranch      bool        `json:"defaultBranch,omitempty"`
	Href               string      `json:"href,omitempty"`
	WebURL             string      `json:"webUrl,omitempty"`
	StatusText         string      `json:"statusText,omitempty"`
	QueuedDate         string      `json:"queuedDate,omitempty"`
	StartDate          string      `json:"startDate,omitempty"`
	FinishDate         string      `json:"finishDate,omitempty"`
	BuildType          *BuildType  `json:"buildType,omitempty"`
	Triggered          *Triggered  `json:"triggered,omitempty"`
	Agent              *Agent      `json:"agent,omitempty"`
	PercentageComplete int         `json:"percentageComplete,omitempty"`
	Pinned             bool        `json:"pinned,omitempty"`
	Tags               *TagList    `json:"tags,omitempty"`
	LastChanges        *ChangeList `json:"lastChanges,omitempty"`
	WaitReason         string      `json:"waitReason,omitempty"`
	UsedByOtherBuilds  bool        `json:"usedByOtherBuilds,omitempty"`
}

// BuildList represents a list of builds
type BuildList struct {
	Count    int     `json:"count"`
	Href     string  `json:"href"`
	NextHref string  `json:"nextHref,omitempty"`
	Builds   []Build `json:"build"`
}

// Triggered represents who/what triggered a build
type Triggered struct {
	Type string `json:"type,omitempty"`
	Date string `json:"date,omitempty"`
	User *User  `json:"user,omitempty"`
}

// Agent represents a build agent
type Agent struct {
	ID         int    `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	TypeID     int    `json:"typeId,omitempty"`
	Connected  bool   `json:"connected,omitempty"`
	Enabled    bool   `json:"enabled,omitempty"`
	Authorized bool   `json:"authorized,omitempty"`
	Href       string `json:"href,omitempty"`
	WebURL     string `json:"webUrl,omitempty"`
	Pool       *Pool  `json:"pool,omitempty"`
	Build      *Build `json:"build,omitempty"`
}

// AgentList represents a list of agents
type AgentList struct {
	Count    int     `json:"count"`
	Href     string  `json:"href"`
	NextHref string  `json:"nextHref,omitempty"`
	Agents   []Agent `json:"agent"`
}

// Pool represents an agent pool
type Pool struct {
	ID        int          `json:"id,omitempty"`
	Name      string       `json:"name,omitempty"`
	Href      string       `json:"href,omitempty"`
	MaxAgents int          `json:"maxAgents,omitempty"`
	Projects  *ProjectList `json:"projects,omitempty"`
	Agents    *AgentList   `json:"agents,omitempty"`
}

// PoolList represents a list of agent pools
type PoolList struct {
	Count int    `json:"count"`
	Pools []Pool `json:"agentPool"`
}

// Compatibility represents build type compatibility info
type Compatibility struct {
	Compatible bool                 `json:"compatible"`
	BuildType  *BuildType           `json:"buildType,omitempty"`
	Agent      *Agent               `json:"agent,omitempty"`
	Reasons    *IncompatibleReasons `json:"incompatibleReasons,omitempty"`
}

// CompatibilityList represents a list of compatibility entries
type CompatibilityList struct {
	Count         int             `json:"count"`
	Compatibility []Compatibility `json:"compatibility"`
}

// IncompatibleReasons contains reasons why an agent can't run a build type
type IncompatibleReasons struct {
	Reasons []string `json:"reason,omitempty"`
}

// QueuedBuild represents a build in the queue
type QueuedBuild struct {
	ID          int        `json:"id"`
	BuildTypeID string     `json:"buildTypeId,omitempty"`
	State       string     `json:"state,omitempty"`
	BranchName  string     `json:"branchName,omitempty"`
	Href        string     `json:"href,omitempty"`
	WebURL      string     `json:"webUrl,omitempty"`
	BuildType   *BuildType `json:"buildType,omitempty"`
	Triggered   *Triggered `json:"triggered,omitempty"`
	QueuedDate  string     `json:"queuedDate,omitempty"`
	WaitReason  string     `json:"waitReason,omitempty"`
}

// BuildQueue represents the build queue
type BuildQueue struct {
	Count  int           `json:"count"`
	Href   string        `json:"href"`
	Builds []QueuedBuild `json:"build"`
}

// TriggerBuildRequest represents a request to trigger a build
type TriggerBuildRequest struct {
	BuildType            BuildTypeRef       `json:"buildType"`
	BranchName           string             `json:"branchName,omitempty"`
	Properties           *PropertyList      `json:"properties,omitempty"`
	Comment              *BuildComment      `json:"comment,omitempty"`
	Personal             bool               `json:"personal,omitempty"`
	TriggeringOptions    *TriggeringOptions `json:"triggeringOptions,omitempty"`
	Agent                *AgentRef          `json:"agent,omitempty"`
	Tags                 *TagList           `json:"tags,omitempty"`
	LastChanges          *LastChanges       `json:"lastChanges,omitempty"`
	Revisions            *Revisions         `json:"revisions,omitempty"`
	SnapshotDependencies *SnapshotDepBuilds `json:"snapshot-dependencies,omitempty"`
}

type SnapshotDepBuilds struct {
	Build []BuildRef `json:"build"`
}

type Revisions struct {
	Revision []Revision `json:"revision"`
}

type Revision struct {
	Version         string              `json:"version"`
	VcsBranchName   string              `json:"vcsBranchName,omitempty"`
	VcsRootInstance *VcsRootInstanceRef `json:"vcs-root-instance,omitempty"`
}

type VcsRootInstanceRef struct {
	VcsRootID string `json:"vcs-root-id"`
}

type VcsRootEntries struct {
	Count        int            `json:"count"`
	VcsRootEntry []VcsRootEntry `json:"vcs-root-entry"`
}

type VcsRootEntry struct {
	ID      string      `json:"id,omitempty"`
	VcsRoot *VcsRootRef `json:"vcs-root,omitempty"`
}

type VcsRootRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// LastChanges represents the changes to include in a build
type LastChanges struct {
	Change []PersonalChange `json:"change"`
}

// PersonalChange represents a personal change (uploaded diff) reference
type PersonalChange struct {
	ID       string `json:"id"`
	Personal bool   `json:"personal,omitempty"`
}

// BuildComment represents a comment on a build
type BuildComment struct {
	Text string `json:"text"`
}

// TriggeringOptions represents options for triggering a build
type TriggeringOptions struct {
	CleanSources              bool `json:"cleanSources,omitempty"`
	RebuildAllDependencies    bool `json:"rebuildAllDependencies,omitempty"`
	QueueAtTop                bool `json:"queueAtTop,omitempty"`
	RebuildFailedOrIncomplete bool `json:"rebuildFailedOrIncompleteDependencies,omitempty"`
}

// AgentRef is a reference to an agent
type AgentRef struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// TagList represents a list of tags
type TagList struct {
	Tag []Tag `json:"tag"`
}

// Tag represents a build tag
type Tag struct {
	Name string `json:"name"`
}

// ApprovalInfo represents approval information for a queued build
type ApprovalInfo struct {
	Status                     string `json:"status"`
	ConfigurationValid         bool   `json:"configurationValid"`
	CanBeApprovedByCurrentUser bool   `json:"canBeApprovedByCurrentUser"`
}

// BuildTypeRef is a reference to a build type
type BuildTypeRef struct {
	ID string `json:"id"`
}

// PropertyList represents a list of properties
type PropertyList struct {
	Property []Property `json:"property"`
}

// Property represents a build property
type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Server represents TeamCity server info
type Server struct {
	Version      string `json:"version"`
	VersionMajor int    `json:"versionMajor"`
	VersionMinor int    `json:"versionMinor"`
	BuildNumber  string `json:"buildNumber"`
	WebURL       string `json:"webUrl"`
}

type Change struct {
	ID       int    `json:"id,omitempty"`
	Version  string `json:"version,omitempty"` // commit SHA
	Username string `json:"username,omitempty"`
	Date     string `json:"date,omitempty"`
	Comment  string `json:"comment,omitempty"`
	WebURL   string `json:"webUrl,omitempty"`
	Files    *Files `json:"files,omitempty"`
}

type ChangeList struct {
	Count  int      `json:"count"`
	Change []Change `json:"change"`
}

type Files struct {
	File []FileChange `json:"file"`
}

type FileChange struct {
	File       string `json:"file"`
	ChangeType string `json:"changeType"` // added, edited, removed
}

type TestOccurrence struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"` // SUCCESS, FAILURE, IGNORED
	Duration   int    `json:"duration,omitempty"`
	Details    string `json:"details,omitempty"`
	NewFailure bool   `json:"newFailure,omitempty"`
	Ignored    bool   `json:"ignored,omitempty"`
	Href       string `json:"href,omitempty"`

	FirstFailed *TestOccurrence `json:"firstFailed,omitempty"`
	Build       *Build          `json:"build,omitempty"`
}

type TestOccurrences struct {
	Count          int              `json:"count"`
	Passed         int              `json:"passed,omitempty"`
	Failed         int              `json:"failed,omitempty"`
	Ignored        int              `json:"ignored,omitempty"`
	TestOccurrence []TestOccurrence `json:"testOccurrence"`
}

type ProblemOccurrence struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Identity string `json:"identity"`
	Details  string `json:"details"`
}

type ProblemOccurrences struct {
	Count             int                 `json:"count"`
	ProblemOccurrence []ProblemOccurrence `json:"problemOccurrence"`
}

// ParseTeamCityTime parses TeamCity's time format (20250710T080607+0000)
func ParseTeamCityTime(s string) (time.Time, error) {
	return time.Parse("20060102T150405-0700", s)
}

// VersionedSettingsStatus represents the sync status of versioned settings
type VersionedSettingsStatus struct {
	Type        string `json:"type,omitempty"`        // info, warning, error
	Message     string `json:"message,omitempty"`     // Human-readable status message
	Timestamp   string `json:"timestamp,omitempty"`   // When the status was recorded
	DslOutdated bool   `json:"dslOutdated,omitempty"` // DSL scripts need regeneration
}

// VersionedSettingsConfig represents the configuration of versioned settings
type VersionedSettingsConfig struct {
	SynchronizationMode string `json:"synchronizationMode,omitempty"` // enabled, disabled
	Format              string `json:"format,omitempty"`              // kotlin, xml
	BuildSettingsMode   string `json:"buildSettingsMode,omitempty"`   // useFromVCS, useCurrentByDefault
	VcsRootID           string `json:"vcsRootId,omitempty"`
	SettingsPath        string `json:"settingsPath,omitempty"`
	AllowUIEditing      bool   `json:"allowUIEditing,omitempty"`
	ShowSettingsChanges bool   `json:"showSettingsChanges,omitempty"`
}

// SnapshotDependency represents a snapshot dependency between build configurations
type SnapshotDependency struct {
	ID              string     `json:"id"`
	SourceBuildType *BuildType `json:"source-buildType,omitempty"`
}

// SnapshotDependencyList represents a list of snapshot dependencies
type SnapshotDependencyList struct {
	Count              int                  `json:"count"`
	SnapshotDependency []SnapshotDependency `json:"snapshot-dependency"`
}

// CloudProfile represents a cloud profile configured in a project
type CloudProfile struct {
	ID              string   `json:"id"`
	Name            string   `json:"name,omitempty"`
	CloudProviderID string   `json:"cloudProviderId,omitempty"`
	Href            string   `json:"href,omitempty"`
	Project         *Project `json:"project,omitempty"`
}

type CloudProfileList struct {
	Count    int            `json:"count"`
	Profiles []CloudProfile `json:"cloudProfile"`
}

// CloudImage represents a cloud image within a cloud profile
type CloudImage struct {
	ID      string        `json:"id"`
	Name    string        `json:"name,omitempty"`
	Href    string        `json:"href,omitempty"`
	Profile *CloudProfile `json:"profile,omitempty"`
	Project *Project      `json:"project,omitempty"`
}

type CloudImageList struct {
	Count  int          `json:"count"`
	Images []CloudImage `json:"cloudImage"`
}

// CloudInstance represents a running cloud instance
type CloudInstance struct {
	ID        string      `json:"id"`
	Name      string      `json:"name,omitempty"`
	State     string      `json:"state,omitempty"`
	StartDate string      `json:"startDate,omitempty"`
	Href      string      `json:"href,omitempty"`
	Image     *CloudImage `json:"image,omitempty"`
	Agent     *Agent      `json:"agent,omitempty"`
}

type CloudInstanceList struct {
	Count     int             `json:"count"`
	Instances []CloudInstance `json:"cloudInstance"`
}

// StartCloudInstanceRequest is the body for starting a cloud instance
type StartCloudInstanceRequest struct {
	Image CloudImageRef `json:"image"`
}

// CloudImageRef is a reference to a cloud image
type CloudImageRef struct {
	ID string `json:"id"`
}

// VcsRoot represents a TeamCity VCS root
type VcsRoot struct {
	ID         string        `json:"id,omitempty"`
	Name       string        `json:"name,omitempty"`
	VcsName    string        `json:"vcsName,omitempty"`
	Href       string        `json:"href,omitempty"`
	Project    *Project      `json:"project,omitempty"`
	Properties *PropertyList `json:"properties,omitempty"`
}

// VcsRootList represents a list of VCS roots
type VcsRootList struct {
	Count   int       `json:"count"`
	VcsRoot []VcsRoot `json:"vcs-root"`
}

// VcsRootsOptions represents options for listing VCS roots
type VcsRootsOptions struct {
	Project string // affectedProject locator
	Limit   int
	Fields  []string
}

// SSHKey represents an SSH key uploaded to a TeamCity project
type SSHKey struct {
	Name      string   `json:"name"`
	Encrypted bool     `json:"encrypted"`
	PublicKey string   `json:"publicKey,omitempty"`
	Project   *Project `json:"project,omitempty"`
}

// SSHKeyList represents a list of SSH keys
type SSHKeyList struct {
	SSHKey []SSHKey `json:"sshKey"`
}

// ProjectFeature represents a project-level feature (connection, etc.)
type ProjectFeature struct {
	ID         string        `json:"id"`
	Type       string        `json:"type"`
	Properties *PropertyList `json:"properties,omitempty"`
}

// ProjectFeatureList represents a list of project features
type ProjectFeatureList struct {
	Count          int              `json:"count"`
	ProjectFeature []ProjectFeature `json:"projectFeature"`
}

// SSHKeyRef references an SSH key by name for test connection requests
type SSHKeyRef struct {
	Name string `json:"name"`
}

// TestConnectionRequest represents a request to test a VCS connection
type TestConnectionRequest struct {
	URL          string     `json:"url"`
	VcsName      string     `json:"vcsName"`
	IsPrivate    bool       `json:"isPrivate"`
	ConnectionID string     `json:"connectionId,omitempty"`
	Username     string     `json:"username,omitempty"`
	Password     string     `json:"password,omitempty"`
	SSHKey       *SSHKeyRef `json:"sshKey,omitempty"`
}

// TestConnectionResult represents the result of a VCS connection test
type TestConnectionResult struct {
	Status string                `json:"status"`
	Errors []TestConnectionError `json:"errors"`
}

// TestConnectionError represents an error from a connection test
type TestConnectionError struct {
	Message           string `json:"message"`
	StackTrace        string `json:"stackTrace,omitempty"`
	AdditionalMessage string `json:"additionalMessage,omitempty"`
}

// Pipeline represents a TeamCity pipeline (YAML configuration)
type Pipeline struct {
	ID            string        `json:"id"`
	Name          string        `json:"name,omitempty"`
	WebURL        string        `json:"webUrl,omitempty"`
	HeadBuildType *BuildTypeRef `json:"headBuildType,omitempty"`
	Jobs          *PipelineJobs `json:"jobs,omitempty"`
	ParentProject *ProjectRef   `json:"parentProject,omitempty"`
	YAML          string        `json:"yaml,omitempty"`
}

// PipelineJobs represents the jobs within a pipeline
type PipelineJobs struct {
	Count int           `json:"count"`
	Job   []PipelineJob `json:"job,omitempty"`
}

// PipelineJob represents a single job in a pipeline (uses YAML keys, not generated IDs)
type PipelineJob struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PipelineList represents a list of pipelines
type PipelineList struct {
	Count     int        `json:"count"`
	Pipelines []Pipeline `json:"pipeline,omitempty"`
}

// ProjectRef is a lightweight reference to a project
type ProjectRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// CreatePipelineRequest represents a request to create a pipeline
type CreatePipelineRequest struct {
	Name    string              `json:"name"`
	YAML    string              `json:"yaml"`
	VcsRoot *PipelineVcsRootRef `json:"vcsRoot,omitempty"`
}

// PipelineVcsRootRef references an existing VCS root for pipeline creation
type PipelineVcsRootRef struct {
	ExternalVcsRootID string `json:"externalVcsRootId"`
}

// PipelinesOptions represents options for listing pipelines
type PipelinesOptions struct {
	Project string
	Limit   int
	Fields  []string
}

// PipelineRun represents pipeline execution metadata on a build
type PipelineRun struct {
	Number   string           `json:"number,omitempty"`
	Pipeline *PipelineRef     `json:"pipeline,omitempty"`
	Jobs     *PipelineRunJobs `json:"jobs,omitempty"`
}

// PipelineRef is a lightweight reference to a pipeline
type PipelineRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// PipelineRunJobs represents jobs within a pipeline run
type PipelineRunJobs struct {
	Count int              `json:"count,omitempty"`
	Job   []PipelineRunJob `json:"job,omitempty"`
}

// PipelineRunJob represents a job within a pipeline run
type PipelineRunJob struct {
	ID    string    `json:"id,omitempty"`
	Name  string    `json:"name,omitempty"`
	Build *BuildRef `json:"build,omitempty"`
}

// BuildRef is a lightweight reference to a build
type BuildRef struct {
	ID int `json:"id,omitempty"`
}

// APIError represents an error from TeamCity's REST API
type APIError struct {
	Message string `json:"message"`
}

// APIErrorResponse represents TeamCity's error response format
type APIErrorResponse struct {
	Errors []APIError `json:"errors"`
}

// XMLAPIError represents a single error in TeamCity's XML error response.
type XMLAPIError struct {
	Message           string `xml:"message" json:"message"`
	AdditionalMessage string `xml:"additionalMessage" json:"additionalMessage,omitempty"`
	StatusText        string `xml:"statusText" json:"statusText,omitempty"`
}

// XMLAPIErrorResponse represents TeamCity's XML error response format.
type XMLAPIErrorResponse struct {
	XMLName xml.Name      `xml:"errors" json:"-"`
	Errors  []XMLAPIError `xml:"error" json:"errors"`
}

// ParseXMLErrors parses a TeamCity XML error response, returning nil if body is not one.
func ParseXMLErrors(body []byte) *XMLAPIErrorResponse {
	trimmed := strings.TrimSpace(string(body))
	if !strings.HasPrefix(trimmed, "<errors") && !strings.HasPrefix(trimmed, "<?xml") {
		return nil
	}

	var xmlErrs XMLAPIErrorResponse
	if err := xml.Unmarshal(body, &xmlErrs); err != nil {
		return nil
	}

	if len(xmlErrs.Errors) == 0 {
		return nil
	}

	return &xmlErrs
}
