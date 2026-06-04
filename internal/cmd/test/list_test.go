package test_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
)

func handleTestOccurrences(ts *cmdtest.TestServer, captured *string, occ api.TestOccurrences) {
	ts.Handle("GET /app/rest/testOccurrences", func(w http.ResponseWriter, r *http.Request) {
		if captured != nil {
			*captured = r.URL.Query().Get("locator")
		}
		cmdtest.JSON(w, occ)
	})
}

func sampleOccurrences() api.TestOccurrences {
	return api.TestOccurrences{
		Count: 1,
		TestOccurrence: []api.TestOccurrence{{
			ID:     "1",
			Name:   "com.example.FooTest.shouldWork",
			Status: "FAILURE",
			Build: &api.Build{
				ID:        42,
				Number:    "100",
				BuildType: &api.BuildType{ID: "Falcon_Build", Name: "Build"},
			},
		}},
	}
}

func TestList_Filters(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantLocator string
	}{
		{"default_failing", []string{"test", "list", "--project", "Falcon"}, "currentlyFailing:true"},
		{"muted", []string{"test", "list", "--project", "Falcon", "--muted"}, "currentlyMuted:true"},
		{"investigated", []string{"test", "list", "--project", "Falcon", "--investigated"}, "currentlyInvestigated:true"},
		{"by_job", []string{"test", "list", "--job", "Falcon_Build"}, "buildType:(id:Falcon_Build)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := cmdtest.NewTestServer(t)
			var locator string
			handleTestOccurrences(ts, &locator, sampleOccurrences())

			out := cmdtest.CaptureOutput(t, ts.Factory, tc.args...)
			assert.Contains(t, locator, tc.wantLocator)
			assert.Contains(t, out, "com.example.FooTest.shouldWork")
			assert.Contains(t, out, "Build")
			assert.Contains(t, out, "#100")
		})
	}
}

func TestList_RequiresScope(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	ts.Handle("GET /app/rest/testOccurrences", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL)
	})

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "a scope is required", "test", "list")
}

func TestList_JSON(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleTestOccurrences(ts, nil, sampleOccurrences())

	out := cmdtest.CaptureOutput(t, ts.Factory, "test", "list", "--project", "Falcon", "--json")
	assert.Contains(t, out, `"name": "com.example.FooTest.shouldWork"`)
	assert.Contains(t, out, `"id": "1"`)
}

func TestList_Empty(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleTestOccurrences(ts, nil, api.TestOccurrences{Count: 0})

	out := cmdtest.CaptureOutput(t, ts.Factory, "test", "list", "--project", "Falcon", "--muted")
	assert.Contains(t, out, "No muted tests in this scope")
}

func TestList_MutuallyExclusiveFilters(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	ts.Handle("GET /app/rest/testOccurrences", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL)
	})

	err := cmdtest.CaptureErr(t, ts.Factory, "test", "list", "--project", "Falcon", "--muted", "--investigated")
	assert.True(t, strings.Contains(err.Error(), "if any flags in the group"))
}
