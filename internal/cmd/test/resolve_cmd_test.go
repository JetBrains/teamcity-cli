package test_test

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_Fixed(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})

	var gotLocator string
	ts.Handle("GET /app/rest/investigations", func(w http.ResponseWriter, r *http.Request) {
		gotLocator = r.URL.Query().Get("locator")
		cmdtest.JSON(w, api.Investigations{Count: 1, Investigation: []api.Investigation{{ID: "inv1", State: "TAKEN"}}})
	})

	var put []api.Investigation
	var mu sync.Mutex
	ts.Handle("PUT /app/rest/investigations/", func(w http.ResponseWriter, r *http.Request) {
		var inv api.Investigation
		require.NoError(t, json.NewDecoder(r.Body).Decode(&inv))
		mu.Lock()
		put = append(put, inv)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "test", "resolve", "FooTest", "--project", "Falcon")

	assert.Contains(t, gotLocator, "test:(id:-99)")
	require.Len(t, put, 1)
	assert.Equal(t, "FIXED", put[0].State)
	assert.Contains(t, out, "Resolved")
}

func TestResolve_GivenUp(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})
	ts.Handle("GET /app/rest/investigations", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Investigations{Count: 1, Investigation: []api.Investigation{{ID: "inv1", State: "TAKEN"}}})
	})

	var put []api.Investigation
	var mu sync.Mutex
	ts.Handle("PUT /app/rest/investigations/", func(w http.ResponseWriter, r *http.Request) {
		var inv api.Investigation
		require.NoError(t, json.NewDecoder(r.Body).Decode(&inv))
		mu.Lock()
		put = append(put, inv)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	cmdtest.CaptureOutput(t, ts.Factory, "test", "resolve", "FooTest", "--job", "Falcon_Build", "--state", "given-up")

	require.Len(t, put, 1)
	assert.Equal(t, "GIVEN_UP", put[0].State)
}

func TestResolve_JSON(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})
	ts.Handle("GET /app/rest/investigations", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Investigations{Count: 1, Investigation: []api.Investigation{{ID: "inv1", State: "TAKEN"}}})
	})
	ts.Handle("PUT /app/rest/investigations/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "test", "resolve", "FooTest", "--project", "Falcon", "--json")
	assert.Contains(t, out, `"resolved": true`)
	assert.Contains(t, out, `"state": "FIXED"`)
}

func TestResolve_NoActiveInvestigation(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})
	ts.Handle("GET /app/rest/investigations", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Investigations{Count: 0})
	})
	ts.Handle("PUT /app/rest/investigations/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected PUT when no investigation exists")
	})

	err := cmdtest.CaptureErr(t, ts.Factory, "test", "resolve", "FooTest", "--project", "Falcon")
	assert.Contains(t, err.Error(), "investigation")
}

func TestResolve_InvalidState(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "invalid --state",
		"test", "resolve", "FooTest", "--project", "Falcon", "--state", "done")
}

func TestResolve_Ambiguous(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts,
		api.TestRef{ID: "-1", Name: "FooTest"},
		api.TestRef{ID: "-2", Name: "FooTest"},
	)
	ts.Handle("PUT /app/rest/investigations/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected PUT for ambiguous name")
	})

	err := cmdtest.CaptureErr(t, ts.Factory, "test", "resolve", "FooTest", "--project", "Falcon")
	assert.Contains(t, err.Error(), "matches 2 tests")
}

func TestResolve_RequiresScope(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "a scope is required", "test", "resolve", "FooTest")
}
