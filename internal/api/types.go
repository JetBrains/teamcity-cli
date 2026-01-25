package api

import "time"

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
	ID          string  `json:"id"`
	Name        string  `json:"name,omitempty"`
	ProjectName string  `json:"projectName,omitempty"`
	ProjectID   string  `json:"projectId,omitempty"`
	Href        string  `json:"href,omitempty"`
	WebURL      string  `json:"webUrl,omitempty"`
	Paused      bool    `json:"paused,omitempty"`
	Project     Project `json:"project,omitempty"`
}

// BuildTypeList represents a list of build configurations
type BuildTypeList struct {
	Count      int         `json:"count"`
	BuildTypes []BuildType `json:"buildType"`
}

// Build represents a TeamCity build
type Build struct {
	ID                 int        `json:"id"`
	BuildTypeID        string     `json:"buildTypeId,omitempty"`
	Number             string     `json:"number,omitempty"`
	Status             string     `json:"status,omitempty"`
	State              string     `json:"state,omitempty"`
	BranchName         string     `json:"branchName,omitempty"`
	DefaultBranch      bool       `json:"defaultBranch,omitempty"`
	Href               string     `json:"href,omitempty"`
	WebURL             string     `json:"webUrl,omitempty"`
	StatusText         string     `json:"statusText,omitempty"`
	QueuedDate         string     `json:"queuedDate,omitempty"`
	StartDate          string     `json:"startDate,omitempty"`
	FinishDate         string     `json:"finishDate,omitempty"`
	BuildType          *BuildType `json:"buildType,omitempty"`
	Triggered          *Triggered `json:"triggered,omitempty"`
	Agent              *Agent     `json:"agent,omitempty"`
	PercentageComplete int        `json:"percentageComplete,omitempty"`
	Pinned             bool       `json:"pinned,omitempty"`
	Tags               *TagList   `json:"tags,omitempty"`
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
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Href string `json:"href,omitempty"`
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
}

// BuildQueue represents the build queue
type BuildQueue struct {
	Count  int           `json:"count"`
	Href   string        `json:"href"`
	Builds []QueuedBuild `json:"build"`
}

// TriggerBuildRequest represents a request to trigger a build
type TriggerBuildRequest struct {
	BuildType         BuildTypeRef       `json:"buildType"`
	BranchName        string             `json:"branchName,omitempty"`
	Properties        *PropertyList      `json:"properties,omitempty"`
	Comment           *BuildComment      `json:"comment,omitempty"`
	Personal          bool               `json:"personal,omitempty"`
	TriggeringOptions *TriggeringOptions `json:"triggeringOptions,omitempty"`
	Agent             *AgentRef          `json:"agent,omitempty"`
	Tags              *TagList           `json:"tags,omitempty"`
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
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"` // SUCCESS, FAILURE, IGNORED
	Duration int    `json:"duration,omitempty"`
	Ignored  bool   `json:"ignored,omitempty"`
	Href     string `json:"href,omitempty"`
}

type TestOccurrences struct {
	Count          int              `json:"count"`
	Passed         int              `json:"passed,omitempty"`
	Failed         int              `json:"failed,omitempty"`
	Ignored        int              `json:"ignored,omitempty"`
	TestOccurrence []TestOccurrence `json:"testOccurrence"`
}

// ParseTeamCityTime parses TeamCity's time format (20250710T080607+0000)
func ParseTeamCityTime(s string) (time.Time, error) {
	return time.Parse("20060102T150405-0700", s)
}

// APIError represents an error from TeamCity's REST API
type APIError struct {
	Message string `json:"message"`
}

// APIErrorResponse represents TeamCity's error response format
type APIErrorResponse struct {
	Errors []APIError `json:"errors"`
}
