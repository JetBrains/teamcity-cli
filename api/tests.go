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
	Muted        bool
	Investigated bool
	Limit        int
	Fields       []string // testOccurrence fields to return (uses TestListFields.Default if empty)
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

	occFields := opts.Fields
	if len(occFields) == 0 {
		occFields = TestListFields.Default
	}
	fields := fmt.Sprintf("count,testOccurrence(%s)", ToAPIFields(occFields))
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

// ResolveTestID resolves a test name to its test id within the given scope (job or project).
// Scoping the lookup to the same scope as the subsequent write keeps resolution consistent
// with the target — a --job write resolves the name within that build configuration, not a
// wider project. It returns an AmbiguousTestError carrying the candidate list when more than
// one test matches.
func (c *Client) ResolveTestID(ctx context.Context, name string, scope ProblemScopeOptions) (string, error) {
	candidates, err := c.lookupTests(ctx, name, scope)
	if err != nil {
		return "", err
	}

	switch len(candidates) {
	case 0:
		return "", &NotFoundError{Resource: "test", ID: name}
	case 1:
		return candidates[0].ID, nil
	default:
		return "", &AmbiguousTestError{Name: name, Candidates: candidates}
	}
}

// lookupTests returns the distinct tests matching name within scope. A job scope resolves
// through /app/rest/testOccurrences: the /app/rest/tests TestLocator has no buildType
// dimension (only affectedProject), whereas TestOccurrenceLocator does — so a job-scoped
// lookup runs against occurrences and dedupes the candidates by test id. Occurrences are
// per build, so a second distinct test id can appear only on a later page; we follow
// nextHref until exhausted (stopping early once two ids prove ambiguity) so resolution
// never mistakes a paginated result for a unique match.
func (c *Client) lookupTests(ctx context.Context, name string, scope ProblemScopeOptions) ([]TestMatch, error) {
	if scope.Job != "" {
		locator := NewLocator().
			AddLocator("test", NewLocator().Add("name", name)).
			AddLocator("buildType", NewLocator().Add("id", scope.Job)).
			AddInt("count", allPageSize)
		fields := "nextHref,testOccurrence(test(id,name))"
		path := fmt.Sprintf("/app/rest/testOccurrences?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fields))

		seen := make(map[string]bool)
		var candidates []TestMatch
		for path != "" {
			var occ TestOccurrences
			if err := c.get(ctx, path, &occ); err != nil {
				return nil, err
			}
			for _, o := range occ.TestOccurrence {
				if o.Test == nil || o.Test.ID == "" || seen[o.Test.ID] {
					continue
				}
				seen[o.Test.ID] = true
				candidates = append(candidates, TestMatch{ID: o.Test.ID, Name: o.Test.Name})
			}
			if len(candidates) > 1 {
				break // ambiguity proven; no need to page further
			}
			path = c.normalizePaginationPath(occ.NextHref)
		}
		return candidates, nil
	}

	locator := NewLocator().Add("name", name)
	if scope.Project != "" {
		locator.AddLocator("affectedProject", NewLocator().Add("id", scope.Project))
	}
	fields := "count,test(id,name)"
	path := fmt.Sprintf("/app/rest/tests?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fields))

	var list TestList
	if err := c.get(ctx, path, &list); err != nil {
		return nil, err
	}

	candidates := make([]TestMatch, len(list.Test))
	for i, t := range list.Test {
		candidates[i] = TestMatch(t)
	}
	return candidates, nil
}
