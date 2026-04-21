package cmdutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

// failureSummaryFixture sets up a mock server returning the given tests and problems,
// then calls printFailureSummary and returns the plain-text (ANSI-stripped) output.
func failureSummaryFixture(t *testing.T, tests api.TestOccurrences, problems api.ProblemOccurrences, statusText string) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/app/rest/testOccurrences"):
			json.NewEncoder(w).Encode(tests)
		case strings.HasPrefix(r.URL.Path, "/app/rest/problemOccurrences"):
			json.NewEncoder(w).Encode(problems)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	var buf bytes.Buffer
	p := &output.Printer{Out: &buf, ErrOut: &buf}
	client := api.NewClient(ts.URL, "test")
	PrintFailureSummary(t.Context(), p, client, "123", "42", "https://tc/build/123", statusText)
	return ansi.Strip(buf.String())
}

func TestPrintFailureSummary(t *testing.T) {
	t.Run("header includes build number and status text", func(t *testing.T) {
		out := failureSummaryFixture(t,
			api.TestOccurrences{},
			api.ProblemOccurrences{},
			"compilation error",
		)
		assert.Contains(t, out, "123")
		assert.Contains(t, out, "#42 failed: compilation error")
		assert.Contains(t, out, "https://tc/build/123")
	})

	t.Run("header without status text omits colon", func(t *testing.T) {
		out := failureSummaryFixture(t,
			api.TestOccurrences{},
			api.ProblemOccurrences{},
			"",
		)
		assert.Contains(t, out, "#42 failed")
		assert.NotContains(t, out, "failed:")
	})

	t.Run("TC_FAILED_TESTS deduped when failed tests section shown", func(t *testing.T) {
		out := failureSummaryFixture(t,
			api.TestOccurrences{
				Count: 1, Failed: 1,
				TestOccurrence: []api.TestOccurrence{{Name: "TestBroken", Status: "FAILURE"}},
			},
			api.ProblemOccurrences{
				Count: 2,
				ProblemOccurrence: []api.ProblemOccurrence{
					{Type: "TC_FAILED_TESTS", Details: "1 test failed"},
					{Type: "TC_EXIT_CODE", Details: "Exit code 1"},
				},
			},
			"",
		)
		assert.Contains(t, out, "TestBroken", "should list the failed test")
		assert.Contains(t, out, "Exit code 1", "non-TC_FAILED_TESTS problem should appear")
		assert.NotContains(t, out, "1 test failed", "TC_FAILED_TESTS detail should be suppressed")
	})

	t.Run("TC_FAILED_TESTS NOT deduped when no test failures", func(t *testing.T) {
		out := failureSummaryFixture(t,
			api.TestOccurrences{Count: 0, Failed: 0},
			api.ProblemOccurrences{
				Count: 1,
				ProblemOccurrence: []api.ProblemOccurrence{
					{Type: "TC_FAILED_TESTS", Details: "3 tests failed"},
				},
			},
			"",
		)
		assert.Contains(t, out, "3 tests failed")
	})

	t.Run("overflow shows remaining count", func(t *testing.T) {
		out := failureSummaryFixture(t,
			api.TestOccurrences{
				Count: 25, Failed: 25,
				TestOccurrence: []api.TestOccurrence{
					{Name: "TestVisible", Status: "FAILURE"},
				},
			},
			api.ProblemOccurrences{},
			"",
		)
		assert.Contains(t, out, "Failed tests (25)")
		assert.Contains(t, out, "TestVisible")
		assert.Contains(t, out, "... and 24 more")
	})

	t.Run("new failure gets (new) annotation", func(t *testing.T) {
		out := failureSummaryFixture(t,
			api.TestOccurrences{
				Count: 1, Failed: 1,
				TestOccurrence: []api.TestOccurrence{
					{Name: "TestNew", Status: "FAILURE", NewFailure: true, Duration: 1500},
				},
			},
			api.ProblemOccurrences{},
			"",
		)
		assert.Contains(t, out, "(new)")
		assert.Contains(t, out, "1s", "duration should be formatted")
	})

	t.Run("old failure gets failing-since annotation", func(t *testing.T) {
		out := failureSummaryFixture(t,
			api.TestOccurrences{
				Count: 1, Failed: 1,
				TestOccurrence: []api.TestOccurrence{
					{
						Name: "TestOld", Status: "FAILURE",
						FirstFailed: &api.TestOccurrence{Build: &api.Build{Number: "38"}},
					},
				},
			},
			api.ProblemOccurrences{},
			"",
		)
		assert.Contains(t, out, "(failing since #38)")
		assert.NotContains(t, out, "(new)")
	})

	t.Run("problem falls back to identity when details empty", func(t *testing.T) {
		out := failureSummaryFixture(t,
			api.TestOccurrences{},
			api.ProblemOccurrences{
				Count: 1,
				ProblemOccurrence: []api.ProblemOccurrence{
					{Type: "TC_EXIT_CODE", Identity: "exitCode:1", Details: ""},
				},
			},
			"",
		)
		assert.Contains(t, out, "exitCode:1")
	})

	t.Run("test details printed line by line", func(t *testing.T) {
		out := failureSummaryFixture(t,
			api.TestOccurrences{
				Count: 1, Failed: 1,
				TestOccurrence: []api.TestOccurrence{
					{Name: "TestAssert", Status: "FAILURE", Details: "expected 1 got 2\nassert failed"},
				},
			},
			api.ProblemOccurrences{},
			"",
		)
		assert.Contains(t, out, "expected 1 got 2")
		assert.Contains(t, out, "assert failed")
	})

	t.Run("graceful when API errors", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(ts.Close)

		var buf bytes.Buffer
		p := &output.Printer{Out: &buf, ErrOut: &buf}
		client := api.NewClient(ts.URL, "test")
		PrintFailureSummary(t.Context(), p, client, "1", "42", "https://tc/build/1", "")
		out := ansi.Strip(buf.String())
		// Should still print header and URL, not panic
		assert.Contains(t, out, "#42 failed")
		assert.Contains(t, out, "https://tc/build/1")
	})
}
