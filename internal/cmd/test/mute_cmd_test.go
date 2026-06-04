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

// handleResolveTest registers the name→id resolution endpoints returning the given tests.
// Project-scoped lookups hit GET /app/rest/tests; job-scoped lookups hit
// GET /app/rest/testOccurrences, so both are served from the same fixture.
func handleResolveTest(ts *cmdtest.TestServer, tests ...api.TestRef) {
	ts.Handle("GET /app/rest/tests", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.TestList{Count: len(tests), Test: tests})
	})
	ts.Handle("GET /app/rest/testOccurrences", func(w http.ResponseWriter, r *http.Request) {
		occ := make([]api.TestOccurrence, len(tests))
		for i, t := range tests {
			occ[i] = api.TestOccurrence{Test: &api.TestDef{ID: t.ID, Name: t.Name}}
		}
		cmdtest.JSON(w, api.TestOccurrences{Count: len(occ), TestOccurrence: occ})
	})
}

func TestMute_CreatesMute(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "com.example.FooTest.flaky"})

	var sent api.Mute
	ts.Handle("POST /app/rest/mutes", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		cmdtest.JSON(w, api.Mute{ID: 7})
	})

	out := cmdtest.CaptureOutput(t, ts.Factory,
		"test", "mute", "com.example.FooTest.flaky", "--project", "Falcon", "--reason", "flaky")

	require.NotNil(t, sent.Target)
	require.NotNil(t, sent.Target.Tests)
	require.Len(t, sent.Target.Tests.Test, 1)
	assert.Equal(t, "-99", sent.Target.Tests.Test[0].ID)
	require.NotNil(t, sent.Resolution)
	assert.Equal(t, "manually", sent.Resolution.Type)
	require.NotNil(t, sent.Assignment)
	assert.Equal(t, "flaky", sent.Assignment.Text)
	require.NotNil(t, sent.Scope)
	require.NotNil(t, sent.Scope.Project)
	assert.Equal(t, "Falcon", sent.Scope.Project.ID)
	assert.Contains(t, out, "Muted")
}

func TestMute_UntilFixed(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})

	var sent api.Mute
	ts.Handle("POST /app/rest/mutes", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		cmdtest.JSON(w, api.Mute{ID: 1})
	})

	cmdtest.CaptureOutput(t, ts.Factory, "test", "mute", "FooTest", "--job", "Falcon_Build", "--until", "fixed")

	require.NotNil(t, sent.Resolution)
	assert.Equal(t, "whenFixed", sent.Resolution.Type)
	require.NotNil(t, sent.Scope.BuildTypes)
	require.Len(t, sent.Scope.BuildTypes.BuildType, 1)
	assert.Equal(t, "Falcon_Build", sent.Scope.BuildTypes.BuildType[0].ID)
}

func TestMute_UntilDate(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})

	var sent api.Mute
	ts.Handle("POST /app/rest/mutes", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		cmdtest.JSON(w, api.Mute{ID: 1})
	})

	cmdtest.CaptureOutput(t, ts.Factory, "test", "mute", "FooTest", "--project", "Falcon", "--until", "2026-01-21")

	require.NotNil(t, sent.Resolution)
	assert.Equal(t, "atTime", sent.Resolution.Type)
	assert.Equal(t, "20260121T000000+0000", sent.Resolution.Time)
}

func TestMute_JSON(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})
	ts.Handle("POST /app/rest/mutes", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Mute{ID: 7})
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "test", "mute", "FooTest", "--project", "Falcon", "--json")
	assert.Contains(t, out, `"id": 7`)
}

func TestMute_Ambiguous(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts,
		api.TestRef{ID: "-1", Name: "FooTest"},
		api.TestRef{ID: "-2", Name: "FooTest"},
	)
	ts.Handle("POST /app/rest/mutes", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected mute write for ambiguous name")
	})

	err := cmdtest.CaptureErr(t, ts.Factory, "test", "mute", "FooTest", "--project", "Falcon")
	assert.Contains(t, err.Error(), "matches 2 tests")
}

func TestMute_RequiresScope(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "a scope is required", "test", "mute", "FooTest")
}

func TestMute_InvalidUntil(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "invalid --until",
		"test", "mute", "FooTest", "--project", "Falcon", "--until", "soon")
}
