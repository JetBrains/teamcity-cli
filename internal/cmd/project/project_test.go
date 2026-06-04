package project_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
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

	rootCmd := cmd.NewCommand(ts.Factory)
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

// TestProjectSettingsStatusParallelFanOut regresses F18/S3: config+status endpoints fetch concurrently, ~delay instead of ~2×delay.
func TestProjectSettingsStatusParallelFanOut(T *testing.T) {
	const delay = 300 * time.Millisecond
	ts := cmdtest.SetupMockClient(T)

	var configCalls, statusCalls atomic.Int32
	ts.Handle("GET /app/rest/projects/id:ParallelProject", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Project{
			ID:     "ParallelProject",
			Name:   "Parallel Project",
			WebURL: ts.URL + "/project.html?projectId=ParallelProject",
		})
	})
	ts.Handle("GET /app/rest/projects/ParallelProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) {
		configCalls.Add(1)
		time.Sleep(delay)
		cmdtest.JSON(w, api.VersionedSettingsConfig{
			SynchronizationMode: "enabled",
			Format:              "kotlin",
			BuildSettingsMode:   "useFromVCS",
			VcsRootID:           "TestVcsRoot",
		})
	})
	ts.Handle("GET /app/rest/projects/ParallelProject/versionedSettings/status", func(w http.ResponseWriter, r *http.Request) {
		statusCalls.Add(1)
		time.Sleep(delay)
		cmdtest.JSON(w, api.VersionedSettingsStatus{
			Type:      "info",
			Message:   "Settings are up to date",
			Timestamp: "Mon Jan 27 10:30:00 UTC 2025",
		})
	})

	start := time.Now()
	cmdtest.RunCmdWithFactory(T, ts.Factory, "project", "settings", "status", "ParallelProject")
	elapsed := time.Since(start)

	assert.Equal(T, int32(1), configCalls.Load(), "config endpoint should be called exactly once")
	assert.Equal(T, int32(1), statusCalls.Load(), "status endpoint should be called exactly once")
	assert.Less(T, elapsed, 2*delay-50*time.Millisecond,
		"parallel fan-out: both calls should overlap, expected ~%s, sequential would be ~%s", delay, 2*delay)
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

// runListSplit executes a command with separate stdout/stderr buffers so the truncation hint can be distinguished from list output.
func runListSplit(t *testing.T, ts *cmdtest.TestServer, args ...string) (stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	f := ts.CloneFactory()
	// NewCommand strips PersistentPreRun (and thus InitOutput), so mirror the
	// --quiet → Printer.Quiet wiring that production does at runtime.
	f.Printer = &output.Printer{Out: &out, ErrOut: &errBuf, Quiet: slices.Contains(args, "--quiet") || slices.Contains(args, "-q")}
	rootCmd := cmd.NewCommand(f)
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errBuf)
	require.NoError(t, rootCmd.Execute())
	return out.String(), errBuf.String()
}

func handleTruncatedProjects(ts *cmdtest.TestServer) {
	ts.Handle("GET /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("locator"), "count:1000") {
			cmdtest.JSON(w, map[string]any{"count": 3, "project": []map[string]string{
				{"id": "P1", "name": "P1"}, {"id": "P2", "name": "P2"}, {"id": "P3", "name": "P3"},
			}})
			return
		}
		cmdtest.JSON(w, map[string]any{
			"count":    2,
			"nextHref": "/app/rest/projects?locator=count:2,start:2",
			"project":  []map[string]string{{"id": "P1", "name": "P1"}, {"id": "P2", "name": "P2"}},
		})
	})
}

const truncationHint = "use --limit 0 to fetch all"

func TestProjectListTruncationHint(T *testing.T) {
	T.Run("finite limit emits hint on stderr only", func(t *testing.T) {
		ts := cmdtest.NewTestServer(t)
		handleTruncatedProjects(ts)

		stdout, stderr := runListSplit(t, ts, "project", "list", "--limit", "2")
		assert.Contains(t, stdout, "P1")
		assert.Contains(t, stdout, "P2")
		assert.NotContains(t, stdout, truncationHint)
		assert.Contains(t, stderr, "Showing only the first 2 results")
		assert.Contains(t, stderr, truncationHint)
	})

	T.Run("limit 0 fetches all without a hint", func(t *testing.T) {
		ts := cmdtest.NewTestServer(t)
		handleTruncatedProjects(ts)

		stdout, stderr := runListSplit(t, ts, "project", "list", "--limit", "0")
		assert.Contains(t, stdout, "P3")
		assert.NotContains(t, stderr, truncationHint)
	})

	T.Run("json output stays clean while hint goes to stderr", func(t *testing.T) {
		ts := cmdtest.NewTestServer(t)
		handleTruncatedProjects(ts)

		stdout, stderr := runListSplit(t, ts, "project", "list", "--limit", "2", "--json")
		var list api.ProjectList
		require.NoError(t, json.Unmarshal([]byte(stdout), &list))
		assert.Equal(t, 2, list.Count)
		assert.Contains(t, stderr, truncationHint)
	})

	T.Run("quiet suppresses the hint", func(t *testing.T) {
		ts := cmdtest.NewTestServer(t)
		handleTruncatedProjects(ts)

		_, stderr := runListSplit(t, ts, "project", "list", "--limit", "2", "--quiet")
		assert.NotContains(t, stderr, truncationHint)
	})
}
