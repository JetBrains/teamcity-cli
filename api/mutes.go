package api

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// ProblemScopeOptions identifies the project or build-config scope of a mute/investigation write.
// Job (buildType) takes precedence over Project when both are set.
type ProblemScopeOptions struct {
	Project string
	Job     string
}

func (s ProblemScopeOptions) hasScope() bool { return s.Project != "" || s.Job != "" }

// scope builds the *ProblemScope payload for a mute/investigation write.
func (s ProblemScopeOptions) scope() *ProblemScope {
	if s.Job != "" {
		return &ProblemScope{BuildTypes: &BuildTypeRefs{BuildType: []BuildType{{ID: s.Job}}}}
	}
	return &ProblemScope{Project: &Project{ID: s.Project}}
}

// appendLocator appends the scope dimension to a mute/investigation list locator.
func (s ProblemScopeOptions) appendLocator(loc *Locator) {
	if s.Job != "" {
		loc.AddLocator("buildType", NewLocator().Add("id", s.Job))
		return
	}
	loc.AddLocator("affectedProject", NewLocator().Add("id", s.Project))
}

// MuteOptions carries the optional reason and resolution policy for a new mute.
type MuteOptions struct {
	Reason         string
	Resolution     string // "manually" (default), "whenFixed", or "atTime"
	ResolutionTime string // TeamCity-formatted timestamp, required when Resolution == "atTime"
}

// CreateMute mutes a test (by resolved id) within a project or build-config scope.
func (c *Client) CreateMute(ctx context.Context, testID string, scope ProblemScopeOptions, opts MuteOptions) (*Mute, error) {
	if !scope.hasScope() {
		return nil, Validation("a scope is required to mute a test", "pass --project or --job")
	}

	mute := Mute{
		Scope:      scope.scope(),
		Target:     &ProblemTarget{Tests: &TestRefs{Test: []TestRef{{ID: testID}}}},
		Resolution: &Resolution{Type: cmp.Or(opts.Resolution, "manually"), Time: opts.ResolutionTime},
	}
	if opts.Reason != "" {
		mute.Assignment = &MuteAssignment{Text: opts.Reason}
	}

	body, err := json.Marshal(mute)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal mute: %w", err)
	}

	var result Mute
	if err := c.post(ctx, "/app/rest/mutes", bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListMutes returns the mutes targeting the given test id within a project or build-config scope.
func (c *Client) ListMutes(ctx context.Context, testID string, scope ProblemScopeOptions) (*Mutes, error) {
	if !scope.hasScope() {
		return nil, Validation("a scope is required to list mutes", "pass --project or --job")
	}

	locator := NewLocator().AddLocator("test", NewLocator().Add("id", testID))
	scope.appendLocator(locator)

	fields := "count,mute(id,scope(project(id),buildTypes(buildType(id))),target(tests(test(id))),resolution(type),assignment(text))"
	path := fmt.Sprintf("/app/rest/mutes?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fields))

	var mutes Mutes
	if err := c.get(ctx, path, &mutes); err != nil {
		return nil, err
	}
	return &mutes, nil
}

// DeleteMute removes a mute by its id.
func (c *Client) DeleteMute(ctx context.Context, muteID int) error {
	path := fmt.Sprintf("/app/rest/mutes/id:%d", muteID)
	return c.doNoContent(ctx, "DELETE", path, nil, "")
}
