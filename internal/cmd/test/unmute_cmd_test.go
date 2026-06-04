package test_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
)

func TestUnmute_LocatesAndDeletes(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})

	var gotLocator string
	ts.Handle("GET /app/rest/mutes", func(w http.ResponseWriter, r *http.Request) {
		gotLocator = r.URL.Query().Get("locator")
		cmdtest.JSON(w, api.Mutes{Count: 1, Mute: []api.Mute{{ID: 42}}})
	})

	var deleted []string
	var mu sync.Mutex
	ts.Handle("DELETE /app/rest/mutes/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		deleted = append(deleted, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "test", "unmute", "FooTest", "--project", "Falcon")

	assert.Contains(t, gotLocator, "test:(id:-99)")
	assert.Contains(t, gotLocator, "affectedProject:(id:Falcon)")
	assert.Equal(t, []string{"/app/rest/mutes/id:42"}, deleted)
	assert.Contains(t, out, "Unmuted")
}

func TestUnmute_NoActiveMute(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})
	ts.Handle("GET /app/rest/mutes", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Mutes{Count: 0})
	})
	ts.Handle("DELETE /app/rest/mutes/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected delete when no mute exists")
	})

	err := cmdtest.CaptureErr(t, ts.Factory, "test", "unmute", "FooTest", "--project", "Falcon")
	assert.True(t, strings.Contains(strings.ToLower(err.Error()), "mute"))
}

func TestUnmute_JSON(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	handleResolveTest(ts, api.TestRef{ID: "-99", Name: "FooTest"})
	ts.Handle("GET /app/rest/mutes", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Mutes{Count: 1, Mute: []api.Mute{{ID: 42}}})
	})
	ts.Handle("DELETE /app/rest/mutes/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "test", "unmute", "FooTest", "--project", "Falcon", "--json")
	assert.Contains(t, out, `"id": 42`)
}

func TestUnmute_RequiresScope(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "a scope is required", "test", "unmute", "FooTest")
}
