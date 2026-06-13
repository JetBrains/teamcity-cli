package pipeline_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
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
	ts.Handle("POST /app/pipeline/schema/generate", func(w http.ResponseWriter, r *http.Request) {
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
	ts.Handle("POST /app/pipeline/schema/generate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html></html>"))
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "pipeline", "schema")
	assert.Contains(t, out, "warning:")
	assert.Contains(t, out, "predate TeamCity 2026.1")

	err := cmdtest.CaptureErr(t, ts.Factory, "pipeline", "schema", "--refresh")
	assert.Contains(t, err.Error(), "schema endpoint not available")
}

func TestPipelineSchemaServerErrorPropagates(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("POST /app/pipeline/schema/generate", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})

	err := cmdtest.CaptureErr(t, ts.Factory, "pipeline", "schema")
	assert.Contains(t, err.Error(), "failed to fetch pipeline schema")
	assert.NotContains(t, err.Error(), "predate TeamCity 2026.1")
}
