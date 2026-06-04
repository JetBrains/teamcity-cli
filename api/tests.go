package api

import (
	"context"
	"fmt"
	"net/url"
)

// TestQueryOptions scopes a cross-build test query to a project or build configuration.
type TestQueryOptions struct {
	Project      string
	Job          string
	Failing      bool
	Muted        bool
	Investigated bool
	Limit        int
}

func (o TestQueryOptions) hasScope() bool {
	return o.Project != "" || o.Job != ""
}

// addScope appends the project (affectedProject) or build-config (buildType) scope to loc.
func (o TestQueryOptions) addScope(loc *Locator) {
	if o.Job != "" {
		loc.AddLocator("buildType", NewLocator().Add("id", o.Job))
		return
	}
	loc.AddLocator("affectedProject", NewLocator().Add("id", o.Project))
}

// ListTests returns currently failing/muted/investigated tests across builds in a project or job.
// A scope (project or job) is required — server-wide queries are rejected.
func (c *Client) ListTests(ctx context.Context, opts TestQueryOptions) (*TestOccurrences, error) {
	if !opts.hasScope() {
		return nil, Validation("a scope is required for cross-build test queries", "pass --project or --job")
	}

	locator := NewLocator()
	switch {
	case opts.Muted:
		locator.Add("currentlyMuted", "true")
	case opts.Investigated:
		locator.Add("currentlyInvestigated", "true")
	default:
		locator.Add("currentlyFailing", "true")
	}
	opts.addScope(locator)
	locator.AddInt("count", opts.Limit)

	fields := "count,testOccurrence(id,name,status,duration,muted,newFailure,build(id,number,buildType(id,name)))"
	path := fmt.Sprintf("/app/rest/testOccurrences?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fields))

	var occ TestOccurrences
	if err := c.get(ctx, path, &occ); err != nil {
		return nil, err
	}
	return &occ, nil
}

// GetTestHistory returns the pass/fail timeline of a named test within a project or job.
// Pass-rate and average-duration are computed by the caller from the returned slice.
func (c *Client) GetTestHistory(ctx context.Context, name string, opts TestQueryOptions) (*TestOccurrences, error) {
	if !opts.hasScope() {
		return nil, Validation("a scope is required for test history", "pass --project or --job")
	}

	locator := NewLocator().AddLocator("test", NewLocator().Add("name", name))
	opts.addScope(locator)
	locator.AddInt("count", opts.Limit)

	fields := "count,testOccurrence(status,duration,muted,newFailure,build(id,number,branchName,startDate,agent(name)))"
	path := fmt.Sprintf("/app/rest/testOccurrences?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fields))

	var occ TestOccurrences
	if err := c.get(ctx, path, &occ); err != nil {
		return nil, err
	}
	return &occ, nil
}

// ResolveTestID resolves a test name to its test id, optionally scoped to a project.
// It returns an AmbiguousTestError carrying the candidate list when more than one test matches.
func (c *Client) ResolveTestID(ctx context.Context, name, projectID string) (string, error) {
	locator := NewLocator().Add("name", name)
	if projectID != "" {
		locator.AddLocator("affectedProject", NewLocator().Add("id", projectID))
	}

	fields := "count,test(id,name)"
	path := fmt.Sprintf("/app/rest/tests?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fields))

	var list TestList
	if err := c.get(ctx, path, &list); err != nil {
		return "", err
	}

	switch len(list.Test) {
	case 0:
		return "", &NotFoundError{Resource: "test", ID: name}
	case 1:
		return list.Test[0].ID, nil
	default:
		candidates := make([]TestMatch, len(list.Test))
		for i, t := range list.Test {
			candidates[i] = TestMatch(t)
		}
		return "", &AmbiguousTestError{Name: name, Candidates: candidates}
	}
}
