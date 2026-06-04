package test_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvestigate_CreatesInvestigation(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "com.example.FooTest.flaky"})

	var sent api.Investigation
	ts.Handle("POST /app/rest/investigations", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		cmdtest.JSON(w, api.Investigation{ID: "id123", State: "TAKEN"})
	})

	out := cmdtest.CaptureOutput(t, ts.Factory,
		"test", "investigate", "com.example.FooTest.flaky", "--project", "Falcon")

	assert.Equal(t, "TAKEN", sent.State)
	require.NotNil(t, sent.Target)
	require.NotNil(t, sent.Target.Tests)
	require.Len(t, sent.Target.Tests.Test, 1)
	assert.Equal(t, "-99", sent.Target.Tests.Test[0].ID)
	require.NotNil(t, sent.Scope)
	require.NotNil(t, sent.Scope.Project)
	assert.Equal(t, "Falcon", sent.Scope.Project.ID)
	assert.Nil(t, sent.Assignee)
	assert.Contains(t, out, "Investigating")
}

func TestInvestigate_WithAssignee(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})

	var sent api.Investigation
	ts.Handle("POST /app/rest/investigations", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		cmdtest.JSON(w, api.Investigation{ID: "id1", State: "TAKEN"})
	})

	out := cmdtest.CaptureOutput(t, ts.Factory,
		"test", "investigate", "FooTest", "--job", "Falcon_Build", "--assignee", "jdoe")

	require.NotNil(t, sent.Assignee)
	assert.Equal(t, "jdoe", sent.Assignee.Username)
	require.NotNil(t, sent.Scope.BuildTypes)
	require.Len(t, sent.Scope.BuildTypes.BuildType, 1)
	assert.Equal(t, "Falcon_Build", sent.Scope.BuildTypes.BuildType[0].ID)
	assert.Contains(t, out, "jdoe")
}

func TestInvestigate_JSON(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})
	ts.Handle("POST /app/rest/investigations", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Investigation{ID: "id123", State: "TAKEN"})
	})

	out := cmdtest.CaptureOutput(t, ts.Factory,
		"test", "investigate", "FooTest", "--project", "Falcon", "--json")
	assert.Contains(t, out, `"id": "id123"`)
	assert.Contains(t, out, `"state": "TAKEN"`)
}

func TestInvestigate_Ambiguous(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts,
		api.TestRef{ID: "-1", Name: "FooTest"},
		api.TestRef{ID: "-2", Name: "FooTest"},
	)
	ts.Handle("POST /app/rest/investigations", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected investigation write for ambiguous name")
	})

	err := cmdtest.CaptureErr(t, ts.Factory, "test", "investigate", "FooTest", "--project", "Falcon")
	assert.Contains(t, err.Error(), "matches 2 tests")
}

func TestInvestigate_RequiresScope(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "a scope is required", "test", "investigate", "FooTest")
}
