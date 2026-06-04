package test_test

import (
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
)

func historyOccurrences() api.TestOccurrences {
	return api.TestOccurrences{
		Count: 3,
		TestOccurrence: []api.TestOccurrence{
			{Status: "SUCCESS", Duration: 1000, Build: &api.Build{Number: "100", BranchName: "main", StartDate: "20260101T120000+0000"}},
			{Status: "FAILURE", Duration: 3000, Build: &api.Build{Number: "99", BranchName: "feature", StartDate: "20260101T110000+0000"}},
			{Status: "SUCCESS", Duration: 2000, Build: &api.Build{Number: "98", StartDate: "20260101T100000+0000"}},
		},
	}
}

func TestHistory_Table(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	var locator string
	handleTestOccurrences(ts, &locator, historyOccurrences())

	out := cmdtest.CaptureOutput(t, ts.Factory, "test", "history", "com.example.FooTest.shouldWork", "--project", "Falcon")

	assert.Contains(t, locator, "test:(name:com.example.FooTest.shouldWork)")
	assert.Contains(t, locator, "affectedProject:(id:Falcon)")
	assert.Contains(t, out, "BUILD")
	assert.Contains(t, out, "#100")
	assert.Contains(t, out, "main")
	// Footer: 2 of 3 passed.
	assert.Contains(t, out, "Pass rate: 67% (2/3)")
	assert.Contains(t, out, "Avg duration:")
}

func TestHistory_ByJob(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	var locator string
	handleTestOccurrences(ts, &locator, historyOccurrences())

	cmdtest.CaptureOutput(t, ts.Factory, "test", "history", "FooTest", "--job", "Falcon_Build")
	assert.Contains(t, locator, "buildType:(id:Falcon_Build)")
}

func TestHistory_RequiresScope(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	ts.Handle("GET /app/rest/testOccurrences", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL)
	})

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "a scope is required", "test", "history", "FooTest")
}

func TestHistory_JSONIsRaw(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleTestOccurrences(ts, nil, historyOccurrences())

	out := cmdtest.CaptureOutput(t, ts.Factory, "test", "history", "FooTest", "--project", "Falcon", "--json")
	// Raw occurrence array, no footer/pass-rate reshaping.
	assert.Contains(t, out, `"status": "FAILURE"`)
	assert.Contains(t, out, `"number": "100"`)
	assert.NotContains(t, out, "Pass rate")
}

func TestHistory_Empty(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleTestOccurrences(ts, nil, api.TestOccurrences{Count: 0})

	out := cmdtest.CaptureOutput(t, ts.Factory, "test", "history", "FooTest", "--project", "Falcon")
	assert.Contains(t, out, "No runs of")
}
