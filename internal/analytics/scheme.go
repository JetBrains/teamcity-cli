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
	GroupSession   = "teamcity.cli.session"
	GroupCommand   = "teamcity.cli.command"
	GroupAPI       = "teamcity.cli.api"
	GroupAuth      = "teamcity.cli.auth"
	GroupBuild     = "teamcity.cli.build"
	GroupAgent     = "teamcity.cli.agent"
	GroupPipeline  = "teamcity.cli.pipeline"
	GroupSkill     = "teamcity.cli.skill"
	GroupWorkspace = "teamcity.cli.workspace"
	GroupMigrate   = "teamcity.cli.migrate"
)

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

// Definition is the source for event registration and fallback validation metadata.
var Definition = &fus.Definition{
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
	Groups: []fus.GroupDefinition{
		sessionGroup(),
		commandGroup(),
		apiGroup(),
		authGroup(),
		buildGroup(),
		agentGroup(),
		pipelineGroup(),
		skillGroup(),
		workspaceGroup(),
		migrateGroup(),
	},
}

var Scheme = buildScheme()

var groupVersion = func() map[string]int {
	versions := make(map[string]int, len(Definition.Groups))
	for _, group := range Definition.Groups {
		versions[group.ID] = group.Version
	}
	return versions
}()

func buildScheme() *fus.Scheme {
	scheme, err := Definition.BuildValidationScheme()
	if err != nil {
		panic(err)
	}
	return scheme
}

func sessionGroup() fus.GroupDefinition {
	return fus.GroupDefinition{
		ID: GroupSession, Version: 1, Type: fus.GroupTypeState, Description: "Session telemetry",
		Events: []fus.EventDefinition{
			{ID: EventInvoked, Description: "CLI session", Fields: []fus.FieldDefinition{
				{Path: "session_id", Rules: []string{fus.RegexpRefExpr(regexpUUID)}, Anonymized: true},
				{Path: "cli_version", Rules: []string{fus.RegexpRefExpr(regexpSemver)}},
				{Path: "server_version", Rules: []string{fus.RegexpRefExpr(regexpServerVersion)}},
				{Path: "os", Rules: []string{fus.EnumRefExpr(enumOS)}},
				{Path: "arch", Rules: []string{fus.EnumRefExpr(enumArch)}},
				{Path: "server_type", Rules: []string{fus.EnumRefExpr(enumServerType)}},
				{Path: "ci_system", Rules: []string{fus.EnumRefExpr(enumCISystem)}},
				{Path: "auth_source", Rules: []string{fus.EnumRefExpr(enumAuthSource)}},
				{Path: "ai_agent", Rules: []string{fus.EnumRefExpr(enumAIAgent)}},
				{Path: "has_linked_project", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
		},
	}
}

func commandGroup() fus.GroupDefinition {
	return fus.GroupDefinition{
		ID: GroupCommand, Version: 3, Type: fus.GroupTypeCounter, Description: "Command telemetry",
		Events: []fus.EventDefinition{
			{ID: EventExecuted, Description: "Command completed", Fields: []fus.FieldDefinition{
				{Path: "session_id", Rules: []string{fus.RegexpRefExpr(regexpUUID)}, Anonymized: true},
				{Path: "command", Rules: []string{fus.EnumRefExpr(enumCommand)}},
				{Path: "source", Rules: []string{fus.EnumRefExpr(enumSource)}},
				{Path: "has_json", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "has_git_context", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "has_link_context", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "flag_count", Rules: []string{fus.RegexpRefExpr(regexpInteger)}},
				{Path: "exit_code", Rules: []string{fus.EnumRefExpr(enumExitCode)}},
				{Path: "duration_ms", Rules: []string{fus.RegexpRefExpr(regexpInteger)}},
				{Path: "error_type", Rules: []string{fus.EnumRefExpr(enumErrorType)}},
			}},
		},
	}
}

func apiGroup() fus.GroupDefinition {
	return fus.GroupDefinition{
		ID: GroupAPI, Version: 1, Type: fus.GroupTypeCounter, Description: "API telemetry",
		Events: []fus.EventDefinition{
			{ID: EventInvoked, Description: "API request", Fields: []fus.FieldDefinition{
				{Path: "method", Rules: []string{fus.EnumRefExpr(enumHTTPMethod)}},
				{Path: "resource", Rules: []string{fus.EnumRefExpr(enumAPIResource)}},
				{Path: "status_code", Rules: []string{fus.RegexpRefExpr(regexpInteger)}},
				{Path: "is_paginated", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "is_slurp", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "had_fields", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "had_input", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
		},
	}
}

func authGroup() fus.GroupDefinition {
	return fus.GroupDefinition{
		ID: GroupAuth, Version: 1, Type: fus.GroupTypeCounter, Description: "Auth telemetry",
		Events: []fus.EventDefinition{
			{ID: EventLoginCompleted, Description: "Login completed", Fields: []fus.FieldDefinition{
				{Path: "method", Rules: []string{fus.EnumExpr("token", "guest")}},
				{Path: "is_success", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "error_type", Rules: []string{fus.EnumRefExpr(enumErrorType)}},
			}},
			{ID: EventLoginAbandoned, Description: "Login abandoned", Fields: []fus.FieldDefinition{
				{Path: "method", Rules: []string{fus.EnumExpr("token", "guest")}},
				{Path: "failed_step", Rules: []string{fus.EnumExpr("server", "token", "verify")}},
			}},
			{ID: EventTokenLoaded, Description: "Token loaded", Fields: []fus.FieldDefinition{
				{Path: "source", Rules: []string{fus.EnumExpr("keyring", "env", "build_properties", "guest")}},
				{Path: "is_expired", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
		},
	}
}

func buildGroup() fus.GroupDefinition {
	return fus.GroupDefinition{
		ID: GroupBuild, Version: 2, Type: fus.GroupTypeCounter, Description: "Build telemetry",
		Events: []fus.EventDefinition{
			{ID: EventStarted, Description: "Build started", Fields: []fus.FieldDefinition{
				{Path: "is_personal", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "has_local_changes", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "has_branch", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "has_revision", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "param_count", Rules: []string{fus.RegexpRefExpr(regexpInteger)}},
				{Path: "is_watched", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "is_dry_run", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
			{ID: EventWatchFinished, Description: "Build watch finished", Fields: []fus.FieldDefinition{
				{Path: "duration_seconds", Rules: []string{fus.RegexpRefExpr(regexpInteger)}},
				{Path: "final_status", Rules: []string{fus.EnumExpr("success", "failure", "error", "canceled")}},
				{Path: "had_logs", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "is_timed_out", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
			{ID: EventLogViewed, Description: "Build log viewed", Fields: []fus.FieldDefinition{
				{Path: "mode", Rules: []string{fus.EnumExpr("full", "failed", "raw", "follow")}},
				{Path: "is_from_job", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
			{ID: EventTestsViewed, Description: "Build tests viewed", Fields: []fus.FieldDefinition{
				{Path: "filter", Rules: []string{fus.EnumExpr("all", "failed", "muted")}},
				{Path: "is_from_job", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
			{ID: EventDiffViewed, Description: "Build diff viewed", Fields: []fus.FieldDefinition{
				{Path: "had_log_diff", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
		},
	}
}

func agentGroup() fus.GroupDefinition {
	return fus.GroupDefinition{
		ID: GroupAgent, Version: 1, Type: fus.GroupTypeCounter, Description: "Agent telemetry",
		Events: []fus.EventDefinition{
			{ID: EventTerminalClosed, Description: "Agent terminal closed", Fields: []fus.FieldDefinition{
				{Path: "duration_seconds", Rules: []string{fus.RegexpRefExpr(regexpInteger)}},
				{Path: "exit_reason", Rules: []string{fus.EnumExpr("user", "timeout", "disconnected", "error")}},
			}},
			{ID: EventExecFinished, Description: "Agent command finished", Fields: []fus.FieldDefinition{
				{Path: "duration_seconds", Rules: []string{fus.RegexpRefExpr(regexpInteger)}},
				{Path: "exit_code", Rules: []string{fus.RegexpRefExpr(regexpInteger)}},
				{Path: "had_timeout", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
			{ID: EventStateChanged, Description: "Agent state changed", Fields: []fus.FieldDefinition{
				{Path: "action", Rules: []string{fus.EnumExpr("enable", "disable", "authorize", "deauthorize", "move", "reboot")}},
			}},
		},
	}
}

func pipelineGroup() fus.GroupDefinition {
	return fus.GroupDefinition{
		ID: GroupPipeline, Version: 1, Type: fus.GroupTypeCounter, Description: "Pipeline telemetry",
		Events: []fus.EventDefinition{
			{ID: EventValidated, Description: "Pipeline validated", Fields: []fus.FieldDefinition{
				{Path: "error_count", Rules: []string{fus.RegexpRefExpr(regexpInteger)}},
				{Path: "warning_count", Rules: []string{fus.RegexpRefExpr(regexpInteger)}},
				{Path: "is_from_file", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "used_cached_schema", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
			{ID: EventCreated, Description: "Pipeline created", Fields: []fus.FieldDefinition{
				{Path: "is_from_file", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
			{ID: EventSynced, Description: "Pipeline synced", Fields: []fus.FieldDefinition{
				{Path: "action", Rules: []string{fus.EnumExpr("push", "pull")}},
			}},
		},
	}
}

func skillGroup() fus.GroupDefinition {
	return fus.GroupDefinition{
		ID: GroupSkill, Version: 1, Type: fus.GroupTypeCounter, Description: "Skill telemetry",
		Events: []fus.EventDefinition{
			{ID: EventManaged, Description: "Skill managed", Fields: []fus.FieldDefinition{
				{Path: "action", Rules: []string{fus.EnumExpr("install", "update", "remove")}},
				{Path: "agent", Rules: []string{fus.EnumRefExpr(enumSkillAgent)}},
				{Path: "scope", Rules: []string{fus.EnumExpr("global", "project")}},
				{Path: "is_auto_detected", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "is_success", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
		},
	}
}

func migrateGroup() fus.GroupDefinition {
	return fus.GroupDefinition{
		ID: GroupMigrate, Version: 1, Type: fus.GroupTypeCounter, Description: "Migrate telemetry",
		Events: []fus.EventDefinition{
			{ID: EventCompleted, Description: "Migration completed", Fields: []fus.FieldDefinition{
				{Path: "source", Rules: []string{fus.EnumExpr(MigrateSourceGitHubActions, MigrateSourceBamboo, MigrateSourceMixed, MigrateSourceOther, MigrateSourceNone)}},
				{Path: "outcome", Rules: []string{fus.EnumExpr(MigrateOutcomeClean, MigrateOutcomePartial, MigrateOutcomeFailed, MigrateOutcomeNothingFound)}},
				{Path: "validation_status", Rules: []string{fus.EnumExpr(MigrateValidationValid, MigrateValidationInvalid, MigrateValidationSkipped)}},
				{Path: "is_dry_run", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
		},
	}
}

func workspaceGroup() fus.GroupDefinition {
	return fus.GroupDefinition{
		ID: GroupWorkspace, Version: 1, Type: fus.GroupTypeCounter, Description: "Workspace telemetry",
		Events: []fus.EventDefinition{
			{ID: EventLinked, Description: "Workspace linked", Fields: []fus.FieldDefinition{
				{Path: "source", Rules: []string{fus.EnumExpr(WorkspaceSourceFlag, WorkspaceSourceAuto, WorkspaceSourceInteractive)}},
				{Path: "is_ambiguous", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
				{Path: "is_subdir", Rules: []string{fus.EnumRefExpr(enumBoolean)}},
			}},
		},
	}
}
