package project_test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
)

func TestSSHList(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	out := cmdtest.CaptureOutput(t, f, "project", "ssh", "list", "--project", "TestProject")
	assert.Contains(t, out, "deploy-key")
	assert.Contains(t, out, "backup-key")
}

func TestSSHGenerate(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	out := cmdtest.CaptureOutput(t, f, "project", "ssh", "generate", "--name", "test-key", "--project", "TestProject")
	assert.Contains(t, out, "Generated SSH key")
	assert.Contains(t, out, "ssh-ed25519")
}

func TestSSHDelete(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	out := cmdtest.CaptureOutput(t, f, "project", "ssh", "delete", "deploy-key", "--force", "--project", "TestProject")
	assert.Contains(t, out, "Deleted SSH key")
}
