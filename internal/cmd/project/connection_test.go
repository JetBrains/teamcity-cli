package project_test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
)

func TestConnectionList(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	out := cmdtest.CaptureOutput(t, f, "project", "connection", "list", "--project", "TestProject")
	assert.Contains(t, out, "PROJECT_EXT_1")
	assert.Contains(t, out, "GitHub App")
}
