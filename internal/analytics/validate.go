package analytics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	fus "github.com/JetBrains/fus-reporting-api-go"
)

// SchemeFinding is a single client-side-validation diagnostic against a sample event.
type SchemeFinding struct {
	Group string
	Event string
	Field string // empty when the diagnostic is on event.id
	Got   string // a "validation.*" sentinel
}

func (f SchemeFinding) String() string {
	loc := f.Group + "/" + f.Event
	if f.Field != "" {
		loc += "." + f.Field
	}
	return loc + " → " + f.Got
}

// SampleEvents returns one canonical event per (group, event_id) covering every declared field.
func SampleEvents() []fus.LogEvent {
	build := "0.0.1"
	now := time.Now().UnixMilli()
	mk := func(group, eventID string, state bool, data map[string]any) fus.LogEvent {
		return fus.LogEvent{
			Recorder: fus.Recorder{ID: RecorderID, Version: RecorderVersion},
			Product:  ProductCode,
			Build:    build,
			Time:     now,
			Group:    fus.EventGroup{ID: group, Version: groupVersion, State: state},
			Event:    fus.EventAction{ID: eventID, Data: data, Count: 1},
		}
	}
	const sid = "00000000-0000-4000-8000-000000000000"
	return []fus.LogEvent{
		mk(GroupSession, EventInvoked, true, map[string]any{
			"session_id":         sid,
			"cli_version":        "0.0.1",
			"server_version":     "2024.12",
			"os":                 "darwin",
			"arch":               "arm64",
			"server_type":        ServerTypeCloud,
			"ci_system":          CINone,
			"auth_source":        AuthSourceNone,
			"ai_agent":           "claude_code",
			"has_linked_project": true,
		}),
		mk(GroupCommand, EventExecuted, false, map[string]any{
			"session_id":       sid,
			"command":          "run.start",
			"source":           SourceAgent,
			"has_json":         true,
			"has_git_context":  false,
			"has_link_context": true,
			"flag_count":       3,
			"exit_code":        "0",
			"duration_ms":      1234,
			"error_type":       ErrorNone,
		}),
		mk(GroupAPI, EventInvoked, false, map[string]any{
			"method": "GET", "resource": "builds", "status_code": 200,
			"is_paginated": false, "is_slurp": false, "had_fields": false, "had_input": false,
		}),
		mk(GroupAuth, EventLoginCompleted, false, map[string]any{
			"method": AuthMethodToken, "is_success": true, "error_type": ErrorNone,
		}),
		mk(GroupAuth, EventLoginAbandoned, false, map[string]any{
			"method": AuthMethodToken, "failed_step": AuthStepVerify,
		}),
		mk(GroupAuth, EventTokenLoaded, false, map[string]any{
			"source": AuthSourceKeyring, "is_expired": false,
		}),
		mk(GroupBuild, EventStarted, false, map[string]any{
			"is_personal": true, "has_local_changes": true, "has_branch": false,
			"has_revision": false, "param_count": 0, "is_watched": true, "is_dry_run": false,
		}),
		mk(GroupBuild, EventWatchFinished, false, map[string]any{
			"duration_seconds": 60, "final_status": BuildStatusSuccess, "had_logs": true, "is_timed_out": false,
		}),
		mk(GroupBuild, EventLogViewed, false, map[string]any{
			"mode": LogModeFailed, "is_from_job": false,
		}),
		mk(GroupBuild, EventTestsViewed, false, map[string]any{
			"filter": TestsFilterFailed, "is_from_job": false,
		}),
		mk(GroupBuild, EventDiffViewed, false, map[string]any{
			"had_log_diff": true,
		}),
		mk(GroupAgent, EventTerminalClosed, false, map[string]any{
			"duration_seconds": 30, "exit_reason": AgentExitUser,
		}),
		mk(GroupAgent, EventExecFinished, false, map[string]any{
			"duration_seconds": 5, "exit_code": 0, "had_timeout": false,
		}),
		mk(GroupAgent, EventStateChanged, false, map[string]any{
			"action": AgentActionEnable,
		}),
		mk(GroupPipeline, EventValidated, false, map[string]any{
			"error_count": 0, "warning_count": 1, "is_from_file": true, "used_cached_schema": true,
		}),
		mk(GroupPipeline, EventCreated, false, map[string]any{
			"is_from_file": true,
		}),
		mk(GroupPipeline, EventSynced, false, map[string]any{
			"action": PipelineActionPush,
		}),
		mk(GroupSkill, EventManaged, false, map[string]any{
			"action": SkillActionInstall, "agent": "claude_code", "scope": SkillScopeProject,
			"is_auto_detected": true, "is_success": true,
		}),
	}
}

// LintScheme runs every SampleEvents entry through the Scheme validator and returns any sentinels emitted.
func LintScheme() ([]SchemeFinding, error) {
	v, err := fus.NewValidator(Scheme)
	if err != nil {
		return nil, fmt.Errorf("build validator: %w", err)
	}
	var findings []SchemeFinding
	for _, e := range SampleEvents() {
		validated, drop := v.Validate(e)
		if drop {
			findings = append(findings, SchemeFinding{
				Group: e.Group.ID, Event: e.Event.ID,
				Got: "dropped (build/version filter)",
			})
			continue
		}
		if isSentinel(validated.Event.ID) {
			findings = append(findings, SchemeFinding{
				Group: e.Group.ID, Event: e.Event.ID, Got: validated.Event.ID,
			})
		}
		keys := make([]string, 0, len(validated.Event.Data))
		for k := range validated.Event.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if isSentinel(k) {
				findings = append(findings, SchemeFinding{
					Group: e.Group.ID, Event: e.Event.ID, Field: "(unknown key)", Got: k,
				})
				continue
			}
			if s, ok := validated.Event.Data[k].(string); ok && isSentinel(s) {
				findings = append(findings, SchemeFinding{
					Group: e.Group.ID, Event: e.Event.ID, Field: k, Got: s,
				})
			}
		}
	}
	return findings, nil
}

func isSentinel(s string) bool {
	return strings.HasPrefix(s, "validation.")
}
