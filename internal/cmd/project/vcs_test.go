package project_test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
)

func TestVcsList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "list", "--project", "TestProject")
	assert.Contains(T, out, "TestProject_Repo")
	assert.Contains(T, out, "My Repo")
	assert.Contains(T, out, "jetbrains.git")
}

func TestVcsListJSON(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "list", "--project", "TestProject", "--json")
	assert.Contains(T, out, `"id"`)
	assert.Contains(T, out, `"count"`)
}

func TestVcsListPlain(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "list", "--project", "TestProject", "--plain")
	assert.Contains(T, out, "TestProject_Repo")
	assert.Contains(T, out, "\t")
}

func TestVcsListDefaultProject(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "list")
	assert.Contains(T, out, "TestProject_Repo")
}

func TestVcsView(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "view", "TestProject_Repo")
	assert.Contains(T, out, "My Repo")
	assert.Contains(T, out, "ID: TestProject_Repo")
	assert.Contains(T, out, "Type: jetbrains.git")
	assert.Contains(T, out, "Project: TestProject")
	assert.Contains(T, out, "URL: https://github.com/org/repo")
	assert.Contains(T, out, "Branch: refs/heads/main")
	assert.Contains(T, out, "Auth Method: PASSWORD")
	assert.Contains(T, out, "Password: ********")
}

func TestVcsViewJSON(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "view", "TestProject_Repo", "--json")
	assert.Contains(T, out, `"id"`)
	assert.Contains(T, out, `"properties"`)
}

func TestVcsViewNotFound(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(T, f, "No VCS root found", "project", "vcs", "view", "NonExistentVcsRoot123456")
}

func TestVcsDelete(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "delete", "TestProject_Repo", "--force")
	assert.Contains(T, out, "Deleted VCS root TestProject_Repo")
}
