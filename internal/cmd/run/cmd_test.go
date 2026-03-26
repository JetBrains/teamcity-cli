package run_test

import (
	"bytes"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testJob     = "TestProject_Build"
	testBuildID = "1"
)

func TestRunList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "list", "--limit", "5")
	cmdtest.RunCmdWithFactory(T, f, "run", "list", "--job", testJob, "--limit", "3")
	cmdtest.RunCmdWithFactory(T, f, "run", "list", "--project", "TestProject", "--status", "success", "--limit", "2")
	cmdtest.RunCmdWithFactory(T, f, "run", "list", "--json", "--limit", "2")
}

func TestRunListBackwardsDateRange(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactoryExpectErr(T, ts.Factory, "is more recent than", "run", "list", "--since", "2020-01-01", "--until", "2019-01-01")
}

func TestRunView(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "view", testBuildID)
	cmdtest.RunCmdWithFactory(T, f, "run", "view", testBuildID, "--json")
}

func TestRunStart(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "start", testJob, "--comment", "CLI test")
}

func TestRunStartWithOptions(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "start", testJob,
		"-P", "key1=val1",
		"-S", "sys.prop=sysval",
		"-E", "ENV_VAR=envval",
		"-m", "Full options test",
		"-t", "test-tag",
		"--clean",
	)
}

func TestRunCancel(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "cancel", testBuildID, "--comment", "Test cleanup")
}

func TestRunLog(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "log", testBuildID)
}

func TestRunArtifacts(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "artifacts", testBuildID)
	cmdtest.RunCmdWithFactory(T, f, "run", "artifacts", testBuildID, "--json")
	cmdtest.RunCmdWithFactory(T, f, "run", "artifacts", "--job", testJob)
	cmdtest.RunCmdWithFactory(T, f, "run", "artifacts", testBuildID, "--path", "logs", "--json")
	cmdtest.RunCmdWithFactoryExpectErr(T, f, "failed to get artifacts", "run", "artifacts", testBuildID, "--path", "nonexistent")
}

func TestRunPinUnpin(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "pin", testBuildID, "--comment", "CLI test pin")
	cmdtest.RunCmdWithFactory(T, f, "run", "unpin", testBuildID)
}

func TestRunTagUntag(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "tag", testBuildID, "cli-test-tag", "another-tag")
	cmdtest.RunCmdWithFactory(T, f, "run", "untag", testBuildID, "cli-test-tag", "another-tag")
}

func TestRunComment(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "comment", testBuildID, "CLI test comment")
	cmdtest.RunCmdWithFactory(T, f, "run", "comment", testBuildID)
	cmdtest.RunCmdWithFactory(T, f, "run", "comment", testBuildID, "--delete")
}

func TestRunChanges(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "changes", testBuildID)
	cmdtest.RunCmdWithFactory(T, f, "run", "changes", testBuildID, "--no-files")
	cmdtest.RunCmdWithFactory(T, f, "run", "changes", testBuildID, "--json")
}

func TestRunTests(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "tests", testBuildID)
	cmdtest.RunCmdWithFactory(T, f, "run", "tests", testBuildID, "--failed")
	cmdtest.RunCmdWithFactory(T, f, "run", "tests", testBuildID, "--json")
}

func TestRunListWithAtMe(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	config.SetUserForServer("http://mock.teamcity.test", "admin")
	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "list", "--user", "@me", "--limit", "5")
}

func TestInvalidStatusFilter(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	rootCmd := cmd.NewRootCmdWithFactory(ts.Factory)
	rootCmd.SetArgs([]string{"run", "list", "--status", "invalid"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	assert.Error(T, err, "expected error for invalid status")
	assert.Contains(T, err.Error(), "invalid status")
}

func TestValidStatusFilter(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	validStatuses := []string{"success", "failure", "running"}
	for _, status := range validStatuses {
		T.Run(status, func(t *testing.T) {
			rootCmd := cmd.NewRootCmdWithFactory(ts.Factory)
			rootCmd.SetArgs([]string{"run", "list", "--status", status, "--limit", "1"})
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			require.NoError(t, err, "expected no error for valid status %s", status)
		})
	}
}
