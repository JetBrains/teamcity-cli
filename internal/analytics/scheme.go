// Package analytics implements the FUS telemetry pipeline for the TeamCity CLI.
package analytics

import (
	fus "github.com/JetBrains/fus-reporting-api-go"
)

const (
	ProductCode     = "TCX"
	RecorderID      = "TCX"
	RecorderVersion = 1
	SchemeVersion   = "1"
)

const (
	GroupSession  = "teamcity.cli.session"
	GroupCommand  = "teamcity.cli.command"
	GroupAPI      = "teamcity.cli.api"
	GroupAuth     = "teamcity.cli.auth"
	GroupBuild    = "teamcity.cli.build"
	GroupAgent    = "teamcity.cli.agent"
	GroupPipeline = "teamcity.cli.pipeline"
	GroupSkill    = "teamcity.cli.skill"
)

const groupVersion = 1

const (
	regexpUUID          = "uuid"
	regexpSemver        = "semver"
	regexpServerVersion = "server_version"
	regexpInteger       = "integer"
	enumBoolean         = "boolean"
	enumOS              = "os"
	enumArch            = "arch"
	enumServerType      = "server_type"
	enumCISystem        = "ci_system"
	enumAuthSource      = "auth_source"
	enumAIAgent         = "ai_agent"
	enumSource          = "source"
	enumExitCode        = "exit_code"
	enumErrorType       = "error_type"
	enumHTTPMethod      = "http_method"
	enumAPIResource     = "api_resource"
	enumCommand         = "command"
	enumSkillAgent      = "skill_agent"
)

// Scheme is the canonical FUS scheme; consumed at runtime and serialized to schema.json for AP registration.
var Scheme = &fus.Scheme{
	Version: SchemeVersion,
	Rules: &fus.SchemeRules{
		Enums: map[string][]string{
			enumBoolean:     {"true", "false"},
			enumOS:          {"darwin", "linux", "windows", "freebsd", "other"},
			enumArch:        {"amd64", "arm64", "386", "other"},
			enumServerType:  {"cloud", "on_prem"},
			enumCISystem:    {"github_actions", "gitlab", "jenkins", "circleci", "buildkite", "azure", "travis", "teamcity", "other", "none"},
			enumAuthSource:  {"keyring", "env", "build_properties", "guest", "none"},
			enumAIAgent:     allAIAgents(),
			enumSource:      {"human", "agent", "ci", "build_step"},
			enumExitCode:    {"0", "1", "2"},
			enumErrorType:   {"auth", "permission", "not_found", "network", "validation", "read_only", "internal", "none"},
			enumHTTPMethod:  {"GET", "POST", "PUT", "DELETE", "PATCH"},
			enumAPIResource: {"builds", "build_types", "projects", "agents", "users", "vcs", "queue", "tests", "problems", "changes", "pipelines", "cloud", "server", "other"},
			enumCommand:     allCommands(),
			enumSkillAgent:  skillAgentEnum(),
		},
		Regexps: map[string]string{
			regexpUUID:          `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			regexpSemver:        `\d+\.\d+\.\d+(?:-[0-9a-zA-Z.-]+)?`,
			regexpServerVersion: `\d{4}\.\d{1,2}(?:\.\d+)?`,
			regexpInteger:       `\d+`,
		},
	},
	Groups: []fus.GroupSchema{
		sessionGroup(),
		commandGroup(),
		apiGroup(),
		authGroup(),
		buildGroup(),
		agentGroup(),
		pipelineGroup(),
		skillGroup(),
	},
}

func sessionGroup() fus.GroupSchema {
	return fus.GroupSchema{
		ID:   GroupSession,
		Type: fus.GroupTypeState,
		Rules: &fus.SchemeRules{
			EventID: []string{fus.EnumExpr(EventInvoked)},
			EventData: map[string][]string{
				"session_id":         {fus.RegexpRefExpr(regexpUUID)},
				"cli_version":        {fus.RegexpRefExpr(regexpSemver)},
				"server_version":     {fus.RegexpRefExpr(regexpServerVersion)},
				"os":                 {fus.EnumRefExpr(enumOS)},
				"arch":               {fus.EnumRefExpr(enumArch)},
				"server_type":        {fus.EnumRefExpr(enumServerType)},
				"ci_system":          {fus.EnumRefExpr(enumCISystem)},
				"auth_source":        {fus.EnumRefExpr(enumAuthSource)},
				"ai_agent":           {fus.EnumRefExpr(enumAIAgent)},
				"has_linked_project": {fus.EnumRefExpr(enumBoolean)},
			},
		},
		AnonymizedFields: []fus.AnonymizedField{
			{Event: EventInvoked, Fields: []string{"session_id"}},
		},
	}
}

func commandGroup() fus.GroupSchema {
	return fus.GroupSchema{
		ID:   GroupCommand,
		Type: fus.GroupTypeCounter,
		Rules: &fus.SchemeRules{
			EventID: []string{fus.EnumExpr(EventExecuted)},
			EventData: map[string][]string{
				"session_id":       {fus.RegexpRefExpr(regexpUUID)},
				"command":          {fus.EnumRefExpr(enumCommand)},
				"source":           {fus.EnumRefExpr(enumSource)},
				"has_json":         {fus.EnumRefExpr(enumBoolean)},
				"has_git_context":  {fus.EnumRefExpr(enumBoolean)},
				"has_link_context": {fus.EnumRefExpr(enumBoolean)},
				"flag_count":       {fus.RegexpRefExpr(regexpInteger)},
				"exit_code":        {fus.EnumRefExpr(enumExitCode)},
				"duration_ms":      {fus.RegexpRefExpr(regexpInteger)},
				"error_type":       {fus.EnumRefExpr(enumErrorType)},
			},
		},
		AnonymizedFields: []fus.AnonymizedField{
			{Event: EventExecuted, Fields: []string{"session_id"}},
		},
	}
}

func apiGroup() fus.GroupSchema {
	return fus.GroupSchema{
		ID:   GroupAPI,
		Type: fus.GroupTypeCounter,
		Rules: &fus.SchemeRules{
			EventID: []string{fus.EnumExpr(EventInvoked)},
			EventData: map[string][]string{
				"method":       {fus.EnumRefExpr(enumHTTPMethod)},
				"resource":     {fus.EnumRefExpr(enumAPIResource)},
				"status_code":  {fus.RegexpRefExpr(regexpInteger)},
				"is_paginated": {fus.EnumRefExpr(enumBoolean)},
				"is_slurp":     {fus.EnumRefExpr(enumBoolean)},
				"had_fields":   {fus.EnumRefExpr(enumBoolean)},
				"had_input":    {fus.EnumRefExpr(enumBoolean)},
			},
		},
	}
}

func authGroup() fus.GroupSchema {
	return fus.GroupSchema{
		ID:   GroupAuth,
		Type: fus.GroupTypeCounter,
		Rules: &fus.SchemeRules{
			EventID: []string{
				fus.EnumExpr(EventLoginCompleted, EventLoginAbandoned, EventTokenLoaded),
			},
			EventData: map[string][]string{
				"method":      {fus.EnumExpr("token", "guest")},
				"is_success":  {fus.EnumRefExpr(enumBoolean)},
				"error_type":  {fus.EnumRefExpr(enumErrorType)},
				"failed_step": {fus.EnumExpr("server", "token", "verify")},
				"source":      {fus.EnumExpr("keyring", "env", "build_properties", "guest")},
				"is_expired":  {fus.EnumRefExpr(enumBoolean)},
			},
		},
	}
}

func buildGroup() fus.GroupSchema {
	return fus.GroupSchema{
		ID:   GroupBuild,
		Type: fus.GroupTypeCounter,
		Rules: &fus.SchemeRules{
			EventID: []string{
				fus.EnumExpr(EventStarted, EventWatchFinished, EventLogViewed, EventTestsViewed, EventDiffViewed),
			},
			EventData: map[string][]string{
				"is_personal":       {fus.EnumRefExpr(enumBoolean)},
				"has_local_changes": {fus.EnumRefExpr(enumBoolean)},
				"has_branch":        {fus.EnumRefExpr(enumBoolean)},
				"has_revision":      {fus.EnumRefExpr(enumBoolean)},
				"param_count":       {fus.RegexpRefExpr(regexpInteger)},
				"is_watched":        {fus.EnumRefExpr(enumBoolean)},
				"is_dry_run":        {fus.EnumRefExpr(enumBoolean)},
				"duration_seconds":  {fus.RegexpRefExpr(regexpInteger)},
				"final_status":      {fus.EnumExpr("success", "failure", "error", "canceled")},
				"had_logs":          {fus.EnumRefExpr(enumBoolean)},
				"is_timed_out":      {fus.EnumRefExpr(enumBoolean)},
				"mode":              {fus.EnumExpr("full", "failed", "raw", "follow")},
				"is_from_job":       {fus.EnumRefExpr(enumBoolean)},
				"filter":            {fus.EnumExpr("all", "failed")},
				"had_log_diff":      {fus.EnumRefExpr(enumBoolean)},
			},
		},
	}
}

func agentGroup() fus.GroupSchema {
	return fus.GroupSchema{
		ID:   GroupAgent,
		Type: fus.GroupTypeCounter,
		Rules: &fus.SchemeRules{
			EventID: []string{fus.EnumExpr(EventTerminalClosed, EventExecFinished, EventStateChanged)},
			EventData: map[string][]string{
				"duration_seconds": {fus.RegexpRefExpr(regexpInteger)},
				"exit_reason":      {fus.EnumExpr("user", "timeout", "disconnected", "error")},
				"exit_code":        {fus.RegexpRefExpr(regexpInteger)},
				"had_timeout":      {fus.EnumRefExpr(enumBoolean)},
				"action":           {fus.EnumExpr("enable", "disable", "authorize", "deauthorize", "move", "reboot")},
			},
		},
	}
}

func pipelineGroup() fus.GroupSchema {
	return fus.GroupSchema{
		ID:   GroupPipeline,
		Type: fus.GroupTypeCounter,
		Rules: &fus.SchemeRules{
			EventID: []string{fus.EnumExpr(EventValidated, EventCreated, EventSynced)},
			EventData: map[string][]string{
				"error_count":        {fus.RegexpRefExpr(regexpInteger)},
				"warning_count":      {fus.RegexpRefExpr(regexpInteger)},
				"is_from_file":       {fus.EnumRefExpr(enumBoolean)},
				"used_cached_schema": {fus.EnumRefExpr(enumBoolean)},
				"action":             {fus.EnumExpr("push", "pull")},
			},
		},
	}
}

func skillGroup() fus.GroupSchema {
	return fus.GroupSchema{
		ID:   GroupSkill,
		Type: fus.GroupTypeCounter,
		Rules: &fus.SchemeRules{
			EventID: []string{fus.EnumExpr(EventManaged)},
			EventData: map[string][]string{
				"action":           {fus.EnumExpr("install", "update", "remove")},
				"agent":            {fus.EnumRefExpr(enumSkillAgent)},
				"scope":            {fus.EnumExpr("global", "project")},
				"is_auto_detected": {fus.EnumRefExpr(enumBoolean)},
				"is_success":       {fus.EnumRefExpr(enumBoolean)},
			},
		},
	}
}
