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
	assert.Contains(T, out, "Git")
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
	assert.Contains(T, out, "Type: Git")
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

func TestVcsCreateAnonymous(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "create",
		"--url", "https://github.com/org/repo.git",
		"--auth", "anonymous",
		"--project", "TestProject",
	)
	assert.Contains(T, out, "Testing connection...")
	assert.Contains(T, out, "Created VCS root")
	assert.Contains(T, out, "TestProject_NewRoot")
}

func TestVcsCreateNoTest(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "create",
		"--url", "https://github.com/org/repo.git",
		"--auth", "anonymous",
		"--no-test",
	)
	assert.NotContains(T, out, "Testing connection")
	assert.Contains(T, out, "Created VCS root")
}

func TestVcsCreateMissingURL(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(T, f, "url", "project", "vcs", "create", "--auth", "anonymous")
}

func TestVcsTest(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "test", "TestProject_Repo")
	assert.Contains(T, out, "Testing connection...")
	assert.Contains(T, out, "Connection to")
}
