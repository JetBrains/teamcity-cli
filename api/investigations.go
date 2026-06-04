package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// CreateInvestigation assigns an investigation (state TAKEN) for a test within a scope.
// assignee is the optional username the investigation is assigned to.
func (c *Client) CreateInvestigation(ctx context.Context, testID string, scope ProblemScopeOptions, assignee string) (*Investigation, error) {
	if !scope.hasScope() {
		return nil, Validation("a scope is required to investigate a test", "pass --project or --job")
	}

	inv := Investigation{
		State:      "TAKEN",
		Scope:      scope.scope(),
		Target:     &ProblemTarget{Tests: &TestRefs{Test: []TestRef{{ID: testID}}}},
		Resolution: &Resolution{Type: "manually"},
	}
	if assignee != "" {
		inv.Assignee = &User{Username: assignee}
	}

	body, err := json.Marshal(inv)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal investigation: %w", err)
	}

	var result Investigation
	if err := c.post(ctx, "/app/rest/investigations", bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ResolveInvestigation transitions the investigation(s) for a test+scope to FIXED or GIVEN_UP.
func (c *Client) ResolveInvestigation(ctx context.Context, testID string, scope ProblemScopeOptions, state string) error {
	if !scope.hasScope() {
		return Validation("a scope is required to resolve an investigation", "pass --project or --job")
	}

	invs, err := c.listInvestigations(ctx, testID, scope)
	if err != nil {
		return err
	}
	if len(invs.Investigation) == 0 {
		return &NotFoundError{Resource: "investigation", ID: testID}
	}

	for i := range invs.Investigation {
		inv := invs.Investigation[i]
		inv.State = state
		body, err := json.Marshal(inv)
		if err != nil {
			return fmt.Errorf("failed to marshal investigation: %w", err)
		}
		// inv.ID is itself the investigation locator (e.g. "buildType:(id:…),test:(id:…)"),
		// so it is used as the path segment directly — there is no "id" locator dimension.
		path := "/app/rest/investigations/" + inv.ID
		if err := c.doNoContent(ctx, "PUT", path, bytes.NewReader(body), "application/json"); err != nil {
			return err
		}
	}
	return nil
}

// listInvestigations returns the active (state:taken) investigations targeting a test id
// within a scope, with full fields so the entity can be round-tripped on a state update.
func (c *Client) listInvestigations(ctx context.Context, testID string, scope ProblemScopeOptions) (*Investigations, error) {
	locator := NewLocator().Add("state", "taken").AddLocator("test", NewLocator().Add("id", testID))
	scope.appendLocator(locator)

	fields := "count,investigation(id,state,assignee(username),scope(project(id),buildTypes(buildType(id))),target(tests(test(id))),resolution(type))"
	path := fmt.Sprintf("/app/rest/investigations?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fields))

	var invs Investigations
	if err := c.get(ctx, path, &invs); err != nil {
		return nil, err
	}
	return &invs, nil
}
