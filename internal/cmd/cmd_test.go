// Unit tests for CLI commands.
// Uses mock API client (see mock_test.go) - no real server required.
package cmd_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testJob     = "TestProject_Build"
	testProject = "TestProject"
	testBuildID = "1"
)

func newRootCmd() *cmd.RootCommand {
	return cmd.NewRootCmd()
}

func runCmd(t *testing.T, args ...string) {
	t.Helper()
	rootCmd := newRootCmd()
	rootCmd.SetArgs(args)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err, "Execute(%v)", args)
}

func runCmdExpectErr(t *testing.T, want string, args ...string) {
	t.Helper()
	rootCmd := newRootCmd()
	rootCmd.SetArgs(args)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.Error(t, err, "expected error for Execute(%v)", args)
	assert.Contains(t, err.Error(), want)
}

func TestConfig(T *testing.T) {
	setupMockClient(T)

	assert.True(T, config.IsConfigured(), "IsConfigured() with env vars")
	assert.NotEmpty(T, config.GetServerURL(), "GetServerURL()")
	assert.NotEmpty(T, config.GetToken(), "GetToken()")
}

func TestProjectList(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "project", "list", "--limit", "5")
	runCmd(T, "project", "list", "--parent", "_Root", "--limit", "3")
	runCmd(T, "project", "list", "--json", "--limit", "2")
}

func TestListLimitValidation(T *testing.T) {
	setupMockClient(T)

	runCmdExpectErr(T, "--limit must be a positive number", "project", "list", "--limit", "0")
	runCmdExpectErr(T, "--limit must be a positive number", "run", "list", "--limit", "-1")
	runCmdExpectErr(T, "--limit must be a positive number", "job", "list", "--limit", "0")
	runCmdExpectErr(T, "--limit must be a positive number", "agent", "list", "--limit", "-5")
}

func TestRunListBackwardsDateRange(T *testing.T) {
	setupMockClient(T)

	runCmdExpectErr(T, "is more recent than", "run", "list", "--since", "2020-01-01", "--until", "2019-01-01")
}

func TestProjectView(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "project", "view", testProject)
	runCmd(T, "project", "view", testProject, "--json")
}

func TestProjectParam(T *testing.T) {
	setupMockClient(T)

	paramName := "TC_CLI_CMD_TEST"

	runCmd(T, "project", "param", "list", testProject)
	runCmd(T, "project", "param", "set", testProject, paramName, "test_value")
	runCmd(T, "project", "param", "get", testProject, paramName)
	runCmd(T, "project", "param", "delete", testProject, paramName)

	// Test secure param
	runCmd(T, "project", "param", "set", testProject, paramName, "secret", "--secure")
	runCmd(T, "project", "param", "delete", testProject, paramName)
}

func TestProjectToken(T *testing.T) {
	setupMockClient(T)

	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"project", "token", "put", testProject, "test-secret-value"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
}

func TestProjectSettingsStatus(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "project", "settings", "status", testProject)
	runCmd(T, "project", "settings", "status", testProject, "--json")
}

func TestProjectSettingsStatusWarning(T *testing.T) {
	ts := setupMockClient(T)
	ts.Handle("GET /app/rest/projects/id:WarningProject", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.Project{
			ID:     "WarningProject",
			Name:   "Warning Project",
			WebURL: ts.URL + "/project.html?projectId=WarningProject",
		})
	})
	runCmd(T, "project", "settings", "status", "WarningProject")
}

func TestProjectSettingsStatusSyncing(T *testing.T) {
	ts := setupMockClient(T)
	ts.Handle("GET /app/rest/projects/id:SyncingProject", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.Project{
			ID:     "SyncingProject",
			Name:   "Syncing Project",
			WebURL: ts.URL + "/project.html?projectId=SyncingProject",
		})
	})
	ts.Handle("GET /app/rest/projects/SyncingProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.VersionedSettingsConfig{
			SynchronizationMode: "enabled",
			Format:              "kotlin",
			BuildSettingsMode:   "useFromVCS",
			VcsRootID:           "TestVcsRoot",
		})
	})
	ts.Handle("GET /app/rest/projects/SyncingProject/versionedSettings/status", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.VersionedSettingsStatus{
			Type:        "info",
			Message:     "Running DSL (incremental compilation disabled)...",
			Timestamp:   "Mon Jan 27 10:30:00 UTC 2025",
			DslOutdated: false,
		})
	})

	runCmd(T, "project", "settings", "status", "SyncingProject")
}

func TestProjectSettingsStatusNotConfigured(T *testing.T) {
	ts := setupMockClient(T)
	ts.Handle("GET /app/rest/projects/id:NoSettingsProject", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.Project{
			ID:     "NoSettingsProject",
			Name:   "No Settings Project",
			WebURL: ts.URL + "/project.html?projectId=NoSettingsProject",
		})
	})
	runCmd(T, "project", "settings", "status", "NoSettingsProject")
}

func TestJobList(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "job", "list", "--limit", "5")
	runCmd(T, "job", "list", "--project", testProject)
	runCmd(T, "job", "list", "--json", "--limit", "2")
}

func TestJobView(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "job", "view", testJob)
	runCmd(T, "job", "view", testJob, "--json")
}

func TestJobPauseResume(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "job", "pause", testJob)
	runCmd(T, "job", "resume", testJob)
}

func TestJobParam(T *testing.T) {
	setupMockClient(T)

	paramName := "TC_CLI_JOB_TEST"

	runCmd(T, "job", "param", "list", testJob)
	runCmd(T, "job", "param", "set", testJob, paramName, "test_value")
	runCmd(T, "job", "param", "get", testJob, paramName)
	runCmd(T, "job", "param", "delete", testJob, paramName)
}

func TestRunList(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "list", "--limit", "5")
	runCmd(T, "run", "list", "--job", testJob, "--limit", "3")
	runCmd(T, "run", "list", "--project", testProject, "--status", "success", "--limit", "2")
	runCmd(T, "run", "list", "--json", "--limit", "2")
}

func TestRunView(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "view", testBuildID)
	runCmd(T, "run", "view", testBuildID, "--json")
}

func TestRunStart(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "start", testJob, "--comment", "CLI test")
}

func TestRunStartWithOptions(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "start", testJob,
		"-P", "key1=val1",
		"-S", "sys.prop=sysval",
		"-E", "ENV_VAR=envval",
		"-m", "Full options test",
		"-t", "test-tag",
		"--clean",
	)
}

func TestRunCancel(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "cancel", testBuildID, "--comment", "Test cleanup")
}

func TestRunLog(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "log", testBuildID)
}

func TestRunArtifacts(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "artifacts", testBuildID)
	runCmd(T, "run", "artifacts", testBuildID, "--json")
	runCmd(T, "run", "artifacts", "--job", testJob)
}

func TestRunPinUnpin(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "pin", testBuildID, "--comment", "CLI test pin")
	runCmd(T, "run", "unpin", testBuildID)
}

func TestRunTagUntag(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "tag", testBuildID, "cli-test-tag", "another-tag")
	runCmd(T, "run", "untag", testBuildID, "cli-test-tag", "another-tag")
}

func TestRunComment(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "comment", testBuildID, "CLI test comment")
	runCmd(T, "run", "comment", testBuildID) // View
	runCmd(T, "run", "comment", testBuildID, "--delete")
}

func TestQueueList(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "queue", "list", "--limit", "10")
	runCmd(T, "queue", "list", "--job", testJob)
	runCmd(T, "queue", "list", "--json")
}

func TestQueueRemove(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "queue", "remove", "100")
}

func TestQueueTop(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "queue", "top", "100")
}

func TestAgentList(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "agent", "list")
	runCmd(T, "agent", "list", "--pool", "Default")
	runCmd(T, "agent", "list", "--connected")
	runCmd(T, "agent", "list", "--enabled")
	runCmd(T, "agent", "list", "--authorized")
	runCmd(T, "agent", "list", "--json")
	runCmd(T, "agent", "list", "--limit", "10")
}

func TestAgentView(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "agent", "view", "1")
	runCmd(T, "agent", "view", "Agent 1", "--json")
}

func TestAgentEnableDisable(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "agent", "enable", "1")
	runCmd(T, "agent", "disable", "Agent 1")
}

func TestAgentAuthorize(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "agent", "authorize", "1")
	runCmd(T, "agent", "deauthorize", "Agent 1")
}

func TestAgentJobs(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "agent", "jobs", "1")
	runCmd(T, "agent", "jobs", "Agent 1", "--json")
	runCmd(T, "agent", "jobs", "1", "--incompatible")
	runCmd(T, "agent", "jobs", "1", "--incompatible", "--json")
}

func TestAgentMove(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "agent", "move", "Agent 1", "0")
}

func TestAgentReboot(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "agent", "reboot", "Agent 1")
	runCmd(T, "agent", "reboot", "1", "--after-build")
}

func TestPoolList(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "pool", "list")
	runCmd(T, "pool", "list", "--json")
}

func TestPoolView(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "pool", "view", "0")
	runCmd(T, "pool", "view", "0", "--json")
}

func TestPoolLinkUnlink(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "pool", "link", "1", "TestProject")
	runCmd(T, "pool", "unlink", "1", "TestProject")
}

func TestAPICommand(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "api", "/app/rest/server")
	runCmd(T, "api", "/app/rest/server", "--silent")
	runCmd(T, "api", "/app/rest/projects")
	runCmd(T, "api", "/app/rest/server", "--include")
	runCmd(T, "api", "/app/rest/server", "--raw")
}

func TestAPICommandWithCustomHeader(T *testing.T) {
	setupMockClient(T)

	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "-H", "Accept: application/xml"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(T, err)
}

func TestRunChanges(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "changes", testBuildID)
	runCmd(T, "run", "changes", testBuildID, "--no-files")
	runCmd(T, "run", "changes", testBuildID, "--json")
}

func TestRunTests(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "run", "tests", testBuildID)
	runCmd(T, "run", "tests", testBuildID, "--failed")
	runCmd(T, "run", "tests", testBuildID, "--json")
}

func TestRunListWithAtMe(T *testing.T) {
	setupMockClient(T)

	config.SetUserForServer("http://mock.teamcity.test", "admin")
	runCmd(T, "run", "list", "--user", "@me", "--limit", "5")
}

// Error handling and edge case tests

func TestInvalidIDs(T *testing.T) {
	setupMockClient(T)

	cases := []struct {
		name string
		args []string
	}{
		{"project", []string{"project", "view", "NonExistentProject123456"}},
		{"job", []string{"job", "view", "NonExistentJob123456"}},
		{"run", []string{"run", "view", "999999999"}},
	}
	for _, tc := range cases {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rootCmd := newRootCmd()
			rootCmd.SetArgs(tc.args)
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			assert.Error(t, err, "expected error for invalid %s ID", tc.name)
		})
	}
}

func TestAPICommandEdgeCases(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "api", "/app/rest/server", "-X", "GET")
	runCmd(T, "api", "/app/rest/server", "-X", "HEAD")
	runCmd(T, "api", "/app/rest/anything")
}

func TestHelpCommands(T *testing.T) {
	T.Parallel()

	commands := [][]string{
		{"--help"},
		{"project", "--help"},
		{"job", "--help"},
		{"run", "--help"},
		{"queue", "--help"},
		{"agent", "--help"},
		{"pool", "--help"},
		{"auth", "--help"},
		{"api", "--help"},
		{"skill", "--help"},
	}
	for _, args := range commands {
		T.Run(args[0], func(t *testing.T) {
			t.Parallel()
			rootCmd := newRootCmd()
			rootCmd.SetArgs(args)
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			require.NoError(t, err, "Execute(%v)", args)
			assert.NotEmpty(t, out.String(), "expected help output for %v", args)
		})
	}
}

func TestUnknownCommand(T *testing.T) {
	T.Parallel()

	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"nonexistent"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	assert.Error(T, err, "expected error for unknown command")
}

func TestGlobalFlags(T *testing.T) {
	setupMockClient(T)

	runCmd(T, "--quiet", "project", "list", "--limit", "1")
	runCmd(T, "--verbose", "project", "list", "--limit", "1")
	runCmd(T, "--no-color", "project", "list", "--limit", "1")
}

func TestUnknownSubcommand(T *testing.T) {
	T.Parallel()

	commands := [][]string{
		{"run", "invalid"},
		{"project", "invalid"},
		{"queue", "invalid"},
		{"job", "invalid"},
		{"agent", "invalid"},
		{"pool", "invalid"},
		{"auth", "invalid"},
		{"skill", "invalid"},
	}

	for _, args := range commands {
		T.Run(args[0], func(t *testing.T) {
			t.Parallel()

			rootCmd := newRootCmd()
			rootCmd.SetArgs(args)
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			assert.Error(t, err, "expected error for unknown subcommand %v", args)
		})
	}
}

func TestParentCommandWithoutSubcommand(T *testing.T) {
	T.Parallel()

	commands := []string{"run", "project", "queue", "job", "agent", "pool", "auth", "skill"}

	for _, cmd := range commands {
		T.Run(cmd, func(t *testing.T) {
			t.Parallel()

			rootCmd := newRootCmd()
			rootCmd.SetArgs([]string{cmd})
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			assert.Error(t, err, "expected error for %s without subcommand", cmd)
			assert.Contains(t, out.String(), "requires a subcommand")
		})
	}
}

func TestInvalidStatusFilter(T *testing.T) {
	setupMockClient(T)

	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"run", "list", "--status", "invalid"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	assert.Error(T, err, "expected error for invalid status")
	assert.Contains(T, err.Error(), "invalid status")
}

func TestValidStatusFilter(T *testing.T) {
	setupMockClient(T)

	validStatuses := []string{"success", "failure", "running"}
	for _, status := range validStatuses {
		T.Run(status, func(t *testing.T) {
			rootCmd := newRootCmd()
			rootCmd.SetArgs([]string{"run", "list", "--status", status, "--limit", "1"})
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			require.NoError(t, err, "expected no error for valid status %s", status)
		})
	}
}
