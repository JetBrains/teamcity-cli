package project_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/require"
)

const testProject = "TestProject"

func TestProjectList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "project", "list", "--limit", "5")
	cmdtest.RunCmdWithFactory(T, f, "project", "list", "--parent", "_Root", "--limit", "3")
	cmdtest.RunCmdWithFactory(T, f, "project", "list", "--json", "--limit", "2")
}

func TestProjectView(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "project", "view", testProject)
	cmdtest.RunCmdWithFactory(T, f, "project", "view", testProject, "--json")
}

func TestProjectParam(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	paramName := "TC_CLI_CMD_TEST"

	cmdtest.RunCmdWithFactory(T, f, "project", "param", "list", testProject)
	cmdtest.RunCmdWithFactory(T, f, "project", "param", "set", testProject, paramName, "test_value")
	cmdtest.RunCmdWithFactory(T, f, "project", "param", "get", testProject, paramName)
	cmdtest.RunCmdWithFactory(T, f, "project", "param", "delete", testProject, paramName)

	cmdtest.RunCmdWithFactory(T, f, "project", "param", "set", testProject, paramName, "secret", "--secure")
	cmdtest.RunCmdWithFactory(T, f, "project", "param", "delete", testProject, paramName)
}

func TestProjectToken(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	rootCmd := cmd.NewRootCmdWithFactory(ts.Factory)
	rootCmd.SetArgs([]string{"project", "token", "put", testProject, "test-secret-value"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
}

func TestProjectSettingsStatus(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "project", "settings", "status", testProject)
	cmdtest.RunCmdWithFactory(T, f, "project", "settings", "status", testProject, "--json")
}

func TestProjectSettingsStatusWarning(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	ts.Handle("GET /app/rest/projects/id:WarningProject", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Project{
			ID:     "WarningProject",
			Name:   "Warning Project",
			WebURL: ts.URL + "/project.html?projectId=WarningProject",
		})
	})
	cmdtest.RunCmdWithFactory(T, ts.Factory, "project", "settings", "status", "WarningProject")
}

func TestProjectSettingsStatusSyncing(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	ts.Handle("GET /app/rest/projects/id:SyncingProject", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Project{
			ID:     "SyncingProject",
			Name:   "Syncing Project",
			WebURL: ts.URL + "/project.html?projectId=SyncingProject",
		})
	})
	ts.Handle("GET /app/rest/projects/SyncingProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.VersionedSettingsConfig{
			SynchronizationMode: "enabled",
			Format:              "kotlin",
			BuildSettingsMode:   "useFromVCS",
			VcsRootID:           "TestVcsRoot",
		})
	})
	ts.Handle("GET /app/rest/projects/SyncingProject/versionedSettings/status", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.VersionedSettingsStatus{
			Type:        "info",
			Message:     "Running DSL (incremental compilation disabled)...",
			Timestamp:   "Mon Jan 27 10:30:00 UTC 2025",
			DslOutdated: false,
		})
	})

	cmdtest.RunCmdWithFactory(T, ts.Factory, "project", "settings", "status", "SyncingProject")
}

func TestProjectSettingsStatusNotConfigured(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	ts.Handle("GET /app/rest/projects/id:NoSettingsProject", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Project{
			ID:     "NoSettingsProject",
			Name:   "No Settings Project",
			WebURL: ts.URL + "/project.html?projectId=NoSettingsProject",
		})
	})
	cmdtest.RunCmdWithFactory(T, ts.Factory, "project", "settings", "status", "NoSettingsProject")
}

func TestProjectTree(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "project", "tree")
	cmdtest.RunCmdWithFactory(T, f, "project", "tree", "_Root")
	cmdtest.RunCmdWithFactory(T, f, "project", "tree", "--no-jobs")
}

func TestProjectTreeSubproject(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	ts.Handle("GET /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.ProjectList{
			Count: 4,
			Projects: []api.Project{
				{ID: "_Root", Name: "Root"},
				{ID: "Parent", Name: "Parent", ParentProjectID: "_Root"},
				{ID: "Child1", Name: "Alpha", ParentProjectID: "Parent"},
				{ID: "Child2", Name: "Beta", ParentProjectID: "Parent"},
			},
		})
	})

	ts.Handle("GET /app/rest/buildTypes", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildTypeList{
			Count: 2,
			BuildTypes: []api.BuildType{
				{ID: "Child1_Build", Name: "Build", ProjectID: "Child1"},
				{ID: "Child2_Test", Name: "Test", ProjectID: "Child2"},
			},
		})
	})

	cmdtest.RunCmdWithFactory(T, ts.Factory, "project", "tree", "Parent")
}

func TestProjectTreeNotFound(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactoryExpectErr(T, ts.Factory, "not found", "project", "tree", "NonExistentProject123456")
}

func TestProjectTreeDepth(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	ts.Handle("GET /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.ProjectList{
			Count: 4,
			Projects: []api.Project{
				{ID: "_Root", Name: "Root"},
				{ID: "Parent", Name: "Parent Project", ParentProjectID: "_Root"},
				{ID: "Child1", Name: "Alpha", ParentProjectID: "Parent"},
				{ID: "Child2", Name: "Beta", ParentProjectID: "Parent"},
			},
		})
	})

	ts.Handle("GET /app/rest/buildTypes", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildTypeList{
			Count: 2,
			BuildTypes: []api.BuildType{
				{ID: "Child1_Build", Name: "Build", ProjectID: "Child1"},
				{ID: "Child2_Test", Name: "Test", ProjectID: "Child2"},
			},
		})
	})

	cmdtest.RunCmdWithFactory(T, ts.CloneFactory(), "project", "tree", "Parent", "--depth", "1")
	cmdtest.RunCmdWithFactory(T, ts.CloneFactory(), "project", "tree", "Parent", "--depth", "2")
}

func runWithBuf(T *testing.T, ts *cmdtest.TestServer, args ...string) string {
	T.Helper()
	var buf bytes.Buffer
	f := ts.CloneFactory()
	f.Printer = &output.Printer{Out: &buf, ErrOut: &buf}
	cmdtest.RunCmdWithFactory(T, f, args...)
	return buf.String()
}

func TestProjectTreeDepthOutput(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	ts.Handle("GET /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.ProjectList{
			Count: 4,
			Projects: []api.Project{
				{ID: "_Root", Name: "Root"},
				{ID: "Parent", Name: "Parent Project", ParentProjectID: "_Root"},
				{ID: "Child1", Name: "Alpha", ParentProjectID: "Parent"},
				{ID: "Child2", Name: "Beta", ParentProjectID: "Parent"},
			},
		})
	})

	ts.Handle("GET /app/rest/buildTypes", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildTypeList{
			Count: 2,
			BuildTypes: []api.BuildType{
				{ID: "Child1_Build", Name: "Build", ProjectID: "Child1"},
				{ID: "Child2_Test", Name: "Test", ProjectID: "Child2"},
			},
		})
	})

	// --depth 1 shows root and its direct children, but not grandchildren (jobs)
	out1 := runWithBuf(T, ts, "project", "tree", "Parent", "--depth", "1")
	require.Contains(T, out1, "Parent Project", "--depth 1 should show root")
	require.Contains(T, out1, "Alpha", "--depth 1 should show direct children")
	require.Contains(T, out1, "Beta", "--depth 1 should show direct children")
	require.NotContains(T, out1, "Build", "--depth 1 should not show grandchildren")
	require.NotContains(T, out1, "Test", "--depth 1 should not show grandchildren")

	// --depth 2 shows children and their jobs
	out2 := runWithBuf(T, ts, "project", "tree", "Parent", "--depth", "2")
	require.Contains(T, out2, "Alpha", "--depth 2 should show children")
	require.Contains(T, out2, "Build", "--depth 2 should show grandchildren")
}
