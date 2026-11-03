package pipeline_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/onboarding"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

func init() { output.NoColor = true }

func TestPipelineList(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "pipeline", "list")
	assert.Contains(t, out, "TestProject_CI")
	assert.Contains(t, out, "CI")
	assert.Contains(t, out, "Test Project")
}

func TestPipelineListJSON(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "pipeline", "list", "--json")
	assert.Contains(t, out, `"count"`)
	assert.Contains(t, out, `"TestProject_CI"`)
}

func TestPipelineView(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "pipeline", "view", "TestProject_CI")
	assert.Contains(t, out, "CI")
	assert.Contains(t, out, "Test Project")
	assert.Contains(t, out, "build")
	assert.Contains(t, out, "Build")
	assert.Contains(t, out, "test")
	assert.Contains(t, out, "Test")
}

func TestPipelineViewJSON(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "pipeline", "view", "TestProject_CI", "--json")
	assert.Contains(t, out, `"id"`)
	assert.Contains(t, out, `"TestProject_CI"`)
}

func TestPipelineSchema(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "pipeline", "schema")
	assert.Contains(t, out, `"type"`)
	assert.Contains(t, out, `"object"`)
	assert.NotContains(t, out, "warning:")
}

func TestPipelineSchemaRefresh(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ts := cmdtest.SetupMockClient(t)
	hits := 0
	ts.Handle("GET /app/pipeline/schema/complete", func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"object","x-hits":` + strconv.Itoa(hits) + `}`))
	})

	cmdtest.CaptureOutput(t, ts.Factory, "pipeline", "schema")
	cmdtest.CaptureOutput(t, ts.Factory, "pipeline", "schema")
	assert.Equal(t, 1, hits)
	cmdtest.CaptureOutput(t, ts.Factory, "pipeline", "schema", "--refresh")
	assert.Equal(t, 2, hits)
}

func TestPipelineSchemaEmbeddedFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/pipeline/schema/complete", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html></html>"))
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "pipeline", "schema")
	assert.Contains(t, out, "warning:")
	assert.Contains(t, out, "complete schema endpoint is unavailable")

	err := cmdtest.CaptureErr(t, ts.Factory, "pipeline", "schema", "--refresh")
	assert.Contains(t, err.Error(), "schema endpoint not available")
}

func TestPipelineSchemaServerErrorPropagates(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/pipeline/schema/complete", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})

	err := cmdtest.CaptureErr(t, ts.Factory, "pipeline", "schema")
	assert.Contains(t, err.Error(), "failed to fetch pipeline schema")
	assert.NotContains(t, err.Error(), "complete schema endpoint is unavailable")
}

// setupPipelineCreate serves POST /app/pipeline and returns the args that create a pipeline from a local YAML file.
func setupPipelineCreate(t *testing.T) (*cmdtest.TestServer, []string) {
	t.Helper()
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("POST /app/pipeline", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "TestProject", r.URL.Query().Get("parentProjectExtId"))
		var req api.CreatePipelineRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		cmdtest.JSON(w, api.Pipeline{ID: "TestProject_App", Name: req.Name, WebURL: ts.URL + "/pipeline/TestProject_App"})
	})
	file := filepath.Join(t.TempDir(), "pipeline.yml")
	require.NoError(t, os.WriteFile(file, []byte("jobs: {}\n"), 0o600))
	return ts, []string{"pipeline", "create", "App", "--project", "TestProject", "--vcs-root", "TestProject_Repo", "--file", file}
}

func TestPipelineCreate(t *testing.T) {
	ts, args := setupPipelineCreate(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, args...)
	assert.Contains(t, out, `Created pipeline "App" (TestProject_App)`)
	assert.Contains(t, out, ts.URL+"/pipeline/TestProject_App")
}

func TestPipelineCreateInsideOnboardingReportsPipeline(t *testing.T) {
	ts, args := setupPipelineCreate(t)
	cmdtest.EnableAnalytics(t)
	t.Setenv(onboarding.EnvOnboarding, "demo")
	rec := ts.HandleOnboarding(http.StatusNoContent)

	cmdtest.RunCmdWithFactory(t, ts.Factory, args...)

	calls := rec.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "pipeline-created", calls[0].Endpoint)
	assert.JSONEq(t, `{"repositoryType":"demo","pipelineId":"TestProject_App"}`, calls[0].Body)
}

func TestPipelineCreateOutsideOnboardingReportsNothing(t *testing.T) {
	ts, args := setupPipelineCreate(t)
	cmdtest.EnableAnalytics(t)
	t.Setenv(onboarding.EnvOnboarding, "")
	rec := ts.HandleOnboarding(http.StatusNoContent)

	cmdtest.RunCmdWithFactory(t, ts.Factory, args...)
	assert.Empty(t, rec.Calls())
}

func TestPipelineCreateInsideOnboardingOptedOut(t *testing.T) {
	ts, args := setupPipelineCreate(t) // sets DO_NOT_TRACK=1
	t.Setenv(onboarding.EnvOnboarding, "own")
	rec := ts.HandleOnboarding(http.StatusNoContent)

	cmdtest.RunCmdWithFactory(t, ts.Factory, args...)
	assert.Empty(t, rec.Calls())
}

func TestPipelineCreateInsideOnboardingIgnoresOlderServer(t *testing.T) {
	ts, args := setupPipelineCreate(t)
	cmdtest.EnableAnalytics(t)
	t.Setenv(onboarding.EnvOnboarding, "own")
	rec := ts.HandleOnboarding(http.StatusNoContent)

	want := cmdtest.CaptureOutput(t, ts.Factory, args...)
	rec.Respond(http.StatusNotFound)
	got := cmdtest.CaptureOutput(t, ts.Factory, args...)

	assert.Equal(t, want, got)
	assert.Len(t, rec.Calls(), 2)
}
