package pipeline_test

import (
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
