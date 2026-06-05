package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// TestOccurrenceQuery describes a query against /app/rest/testOccurrences.
// Its fields map 1:1 to TeamCity testOccurrences locator dimensions, making it
// the single source of truth for building those locators.
type TestOccurrenceQuery struct {
	// Subject dimensions — at least one of Build or TestName must be set.
	// buildType is only accepted by the finder alongside a test name (it scopes
	// a test's history to one build configuration); on its own it is rejected.
	Build     string // build:(id:…)
	BuildType string // buildType:(id:…) — narrows TestName history to a job
	TestName  string // test:(name:…)

	// Filters.
	Status string // "", passed, failed, ignored, new
	Muted  *bool  // muted:

	Limit  int      // max occurrences (<=0 ⇒ all, paging through nextHref)
	Fields []string // testOccurrence(...) field override (defaults to a lean set)
}

var validTestStatuses = map[string]bool{"": true, "passed": true, "failed": true, "ignored": true, "new": true}

// ValidTestStatus reports whether s is an accepted --status value (including empty).
func ValidTestStatus(s string) bool { return validTestStatuses[s] }

// defaultTestOccurrenceFields is the lean projection used when Query.Fields is empty.
const defaultTestOccurrenceFields = "id,name,status,duration,muted,newFailure,build(id,number,branchName,startDate)"

// buildLocator maps the query's fields onto testOccurrences locator dimensions; count is applied by ListTestOccurrences.
func (q TestOccurrenceQuery) buildLocator() (*Locator, error) {
	if !validTestStatuses[q.Status] {
		return nil, Validation(fmt.Sprintf("invalid status %q", q.Status), "use one of: passed, failed, ignored, new")
	}

	l := NewLocator()
	l.AddLocator("build", NewLocator().Add("id", q.Build))
	l.AddLocator("buildType", NewLocator().Add("id", q.BuildType))
	l.AddLocator("test", NewLocator().Add("name", q.TestName))
	if l.IsEmpty() {
		return nil, Validation("no test scope specified", "set a build or test name")
	}

	switch q.Status {
	case "failed":
		l.AddUpper("status", "FAILURE")
	case "passed":
		l.AddUpper("status", "SUCCESS")
	case "ignored":
		l.Add("ignored", "true")
	case "new":
		l.Add("newFailure", "true")
	}
	if q.Muted != nil {
		l.Add("muted", strconv.FormatBool(*q.Muted))
	}

	return l, nil
}

// ListTestOccurrences probes the aggregate summary, then pages through matching occurrences via nextHref; Limit<=0 fetches all.
func (c *Client) ListTestOccurrences(ctx context.Context, q TestOccurrenceQuery) (*TestOccurrences, error) {
	locator, err := q.buildLocator()
	if err != nil {
		return nil, err
	}

	summaryFields := "count,passed,failed,ignored,muted"
	summaryPath := fmt.Sprintf("/app/rest/testOccurrences?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(summaryFields))

	var summary TestOccurrences
	if err := c.get(ctx, summaryPath, &summary); err != nil {
		return nil, err
	}

	limit := max(q.Limit, 0)

	inner := defaultTestOccurrenceFields
	if len(q.Fields) > 0 {
		inner = strings.Join(q.Fields, ",")
	}
	detailFields := "count,nextHref,testOccurrence(" + inner + ")"

	locator.AddInt("count", pageCount(limit))
	detailPath := fmt.Sprintf("/app/rest/testOccurrences?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(detailFields))

	occurrences, _, err := collectPages(c, detailPath, limit, func(p string) ([]TestOccurrence, string, error) {
		var page TestOccurrences
		if err := c.get(ctx, p, &page); err != nil {
			return nil, "", err
		}
		return page.TestOccurrence, page.NextHref, nil
	})
	if err != nil {
		return nil, err
	}

	summary.TestOccurrence = occurrences
	return &summary, nil
}
