package cmd_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/tiulpin/teamcity-cli/internal/api"
	"github.com/tiulpin/teamcity-cli/internal/cmd"
	"github.com/tiulpin/teamcity-cli/internal/config"
)

var (
	testJob     string
	testProject string
	testBuild   *api.Build
)

func TestMain(m *testing.M) {
	godotenv.Load("../../.env")

	url := os.Getenv("TEAMCITY_URL")
	token := os.Getenv("TEAMCITY_TOKEN")
	testJob = os.Getenv("TEAMCITY_TEST_CONFIG")
	testProject = os.Getenv("TEAMCITY_TEST_PROJECT")

	if url == "" || token == "" {
		println("Skipping integration tests: TEAMCITY_URL or TEAMCITY_TOKEN not set")
		os.Exit(0)
	}

	config.Init()

	client := newTestClient()
	if err := ensureTestBuild(client); err != nil {
		println("Warning: could not ensure test build:", err.Error())
	}

	if user, err := client.GetCurrentUser(); err == nil {
		config.SetUserForServer(os.Getenv("TEAMCITY_URL"), user.Username)
		println("Set current user:", user.Username)
	}

	os.Exit(m.Run())
}

// ensureTestBuild ensures a finished build exists for tests that require one.
// It first checks for an existing finished build, and if none exists,
// triggers a new build and waits for it to complete.
func ensureTestBuild(client *api.Client) error {
	builds, err := client.GetBuilds(api.BuildsOptions{
		BuildTypeID: testJob,
		State:       "finished",
		Limit:       1,
	})
	if err != nil {
		return fmt.Errorf("failed to check for existing builds: %w", err)
	}

	if builds.Count > 0 {
		testBuild = &builds.Builds[0]
		println("Using existing finished build:", testBuild.ID)
		return nil
	}

	println("No finished builds found, triggering a new build...")
	build, err := client.RunBuild(testJob, api.RunBuildOptions{
		Comment: "Integration test setup - ensuring test data exists",
	})
	if err != nil {
		return fmt.Errorf("failed to trigger build: %w", err)
	}
	println("Triggered build:", build.ID)

	timeout := 5 * time.Minute
	pollInterval := 5 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		build, err = client.GetBuild(fmt.Sprintf("%d", build.ID))
		if err != nil {
			return fmt.Errorf("failed to get build status: %w", err)
		}

		if build.State == "finished" {
			testBuild = build
			println("Build finished with status:", build.Status)
			return nil
		}

		println("Build state:", build.State, "- waiting...")
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("build did not finish within %v", timeout)
}

func newTestClient() *api.Client {
	return api.NewClient(os.Getenv("TEAMCITY_URL"), os.Getenv("TEAMCITY_TOKEN"))
}

func newRootCmd() *cmd.RootCommand {
	return cmd.GetRootCmd()
}

func runCmd(t *testing.T, args ...string) {
	t.Helper()
	rootCmd := newRootCmd()
	rootCmd.SetArgs(args)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Command %v failed: %v", args, err)
	}
}

func TestConfig(t *testing.T) {
	if !config.IsConfigured() {
		t.Error("Expected config to be configured with env vars")
	}
	if config.GetServerURL() == "" {
		t.Error("Expected server URL to be set")
	}
	if config.GetToken() == "" {
		t.Error("Expected token to be set")
	}
}

func TestProjectList(t *testing.T) {
	runCmd(t, "project", "list", "--limit", "5")
	runCmd(t, "project", "list", "--parent", "_Root", "--limit", "3")
	runCmd(t, "project", "list", "--json", "--limit", "2")
}

func TestProjectView(t *testing.T) {
	runCmd(t, "project", "view", testProject)
	runCmd(t, "project", "view", testProject, "--json")
}

func TestProjectParam(t *testing.T) {
	paramName := "TC_CLI_CMD_TEST"

	runCmd(t, "project", "param", "list", testProject)
	runCmd(t, "project", "param", "set", testProject, paramName, "test_value")
	runCmd(t, "project", "param", "get", testProject, paramName)
	runCmd(t, "project", "param", "delete", testProject, paramName)

	// Test secure param
	runCmd(t, "project", "param", "set", testProject, paramName, "secret", "--secure")
	runCmd(t, "project", "param", "delete", testProject, paramName)
}

func TestProjectToken(t *testing.T) {
	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"project", "token", "create", testProject, "test-secret-value"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	// Token commands may fail due to permissions - that's ok
	if err := rootCmd.Execute(); err != nil {
		t.Logf("Token create (may need permission): %v", err)
	}
}

func TestJobList(t *testing.T) {
	runCmd(t, "job", "list", "--limit", "5")
	runCmd(t, "job", "list", "--project", testProject)
	runCmd(t, "job", "list", "--json", "--limit", "2")
}

func TestJobView(t *testing.T) {
	runCmd(t, "job", "view", testJob)
	runCmd(t, "job", "view", testJob, "--json")
}

func TestJobPauseResume(t *testing.T) {
	runCmd(t, "job", "pause", testJob)
	runCmd(t, "job", "resume", testJob)
}

func TestJobParam(t *testing.T) {
	paramName := "TC_CLI_JOB_TEST"

	runCmd(t, "job", "param", "list", testJob)
	runCmd(t, "job", "param", "set", testJob, paramName, "test_value")
	runCmd(t, "job", "param", "get", testJob, paramName)
	runCmd(t, "job", "param", "delete", testJob, paramName)
}

func TestRunList(t *testing.T) {
	runCmd(t, "run", "list", "--limit", "5")
	runCmd(t, "run", "list", "--job", testJob, "--limit", "3")
	runCmd(t, "run", "list", "--project", testProject, "--status", "success", "--limit", "2")
	runCmd(t, "run", "list", "--json", "--limit", "2")
}

func TestRunView(t *testing.T) {
	if testBuild == nil {
		t.Skip("No test build available")
	}

	runID := fmt.Sprintf("%d", testBuild.ID)
	runCmd(t, "run", "view", runID)
	runCmd(t, "run", "view", runID, "--json")
}

func TestRunStartAndCancel(t *testing.T) {
	client := newTestClient()

	runCmd(t, "run", "start", testJob, "--comment", "CLI test")

	builds, _ := client.GetBuildQueue(api.QueueOptions{BuildTypeID: testJob, Limit: 1})
	if builds != nil && builds.Count > 0 {
		runID := fmt.Sprintf("%d", builds.Builds[0].ID)
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"run", "cancel", runID, "--comment", "Test cleanup"})
		rootCmd.Execute() // May fail if build already started
	}
}

func TestRunStartWithOptions(t *testing.T) {
	client := newTestClient()

	runCmd(t, "run", "start", testJob,
		"-P", "key1=val1",
		"-S", "sys.prop=sysval",
		"-E", "ENV_VAR=envval",
		"-m", "Full options test",
		"-t", "test-tag",
		"--clean",
	)

	builds, _ := client.GetBuildQueue(api.QueueOptions{BuildTypeID: testJob, Limit: 1})
	if builds != nil && builds.Count > 0 {
		client.RemoveFromQueue(fmt.Sprintf("%d", builds.Builds[0].ID))
	}
}

func TestRunLog(t *testing.T) {
	if testBuild == nil {
		t.Skip("No test build available")
	}

	runID := fmt.Sprintf("%d", testBuild.ID)
	runCmd(t, "run", "log", runID)
}

func TestRunDownload(t *testing.T) {
	if testBuild == nil {
		t.Skip("No test build available")
	}

	runID := fmt.Sprintf("%d", testBuild.ID)
	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"run", "download", runID, "--dir", "/tmp/tc-test-artifacts"})
	// May fail if no artifacts - that's ok
	rootCmd.Execute()
}

func TestRunPinUnpin(t *testing.T) {
	if testBuild == nil {
		t.Skip("No test build available")
	}

	runID := fmt.Sprintf("%d", testBuild.ID)
	runCmd(t, "run", "pin", runID, "--comment", "CLI test pin")
	runCmd(t, "run", "unpin", runID)
}

func TestRunTagUntag(t *testing.T) {
	if testBuild == nil {
		t.Skip("No test build available")
	}

	runID := fmt.Sprintf("%d", testBuild.ID)
	runCmd(t, "run", "tag", runID, "cli-test-tag", "another-tag")
	time.Sleep(500 * time.Millisecond) // Wait for API eventual consistency
	runCmd(t, "run", "untag", runID, "cli-test-tag", "another-tag")
}

func TestRunComment(t *testing.T) {
	if testBuild == nil {
		t.Skip("No test build available")
	}

	runID := fmt.Sprintf("%d", testBuild.ID)
	runCmd(t, "run", "comment", runID, "CLI test comment")
	runCmd(t, "run", "comment", runID) // View
	runCmd(t, "run", "comment", runID, "--delete")
}

func TestQueueList(t *testing.T) {
	runCmd(t, "queue", "list", "--limit", "10")
	runCmd(t, "queue", "list", "--job", testJob)
	runCmd(t, "queue", "list", "--json")
}

func TestQueueOperations(t *testing.T) {
	client := newTestClient()

	// Test queue remove
	t.Run("remove", func(t *testing.T) {
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"run", "start", testJob, "-m", "Queue remove test"})
		rootCmd.Execute()

		builds, _ := client.GetBuildQueue(api.QueueOptions{BuildTypeID: testJob, Limit: 1})
		if builds == nil || builds.Count == 0 {
			t.Skip("No runs in queue")
		}

		runID := fmt.Sprintf("%d", builds.Builds[0].ID)
		rootCmd = newRootCmd()
		rootCmd.SetArgs([]string{"queue", "remove", runID})
		rootCmd.Execute() // May fail if already started
	})

	// Test queue top
	t.Run("top", func(t *testing.T) {
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"run", "start", testJob, "-m", "Queue top test"})
		rootCmd.Execute()

		builds, _ := client.GetBuildQueue(api.QueueOptions{BuildTypeID: testJob, Limit: 1})
		if builds == nil || builds.Count == 0 {
			t.Skip("No runs in queue")
		}

		runID := fmt.Sprintf("%d", builds.Builds[0].ID)
		rootCmd = newRootCmd()
		rootCmd.SetArgs([]string{"queue", "top", runID})
		rootCmd.Execute() // May fail if already started

		client.CancelBuild(runID, "Test cleanup")
	})

	// Test queue approve
	t.Run("approve", func(t *testing.T) {
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"run", "start", testJob, "-m", "Queue approve test"})
		rootCmd.Execute()

		builds, _ := client.GetBuildQueue(api.QueueOptions{BuildTypeID: testJob, Limit: 1})
		if builds == nil || builds.Count == 0 {
			t.Skip("No runs in queue")
		}

		runID := fmt.Sprintf("%d", builds.Builds[0].ID)
		rootCmd = newRootCmd()
		rootCmd.SetArgs([]string{"queue", "approve", runID})
		rootCmd.Execute() // May fail if approval not required

		client.CancelBuild(runID, "Test cleanup")
	})
}

func TestAPICommand(t *testing.T) {
	// Test GET server info
	runCmd(t, "api", "/app/rest/server")

	// Test GET with silent mode
	runCmd(t, "api", "/app/rest/server", "--silent")

	// Test GET projects
	runCmd(t, "api", "/app/rest/projects")

	// Test with include headers
	runCmd(t, "api", "/app/rest/server", "--include")

	// Test with raw output
	runCmd(t, "api", "/app/rest/server", "--raw")
}

func TestAPICommandWithCustomHeader(t *testing.T) {
	// Test with Accept header for XML
	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "-H", "Accept: application/xml"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("API command with custom header failed: %v", err)
	}
}

func TestAPICommandMethod(t *testing.T) {
	runCmd(t, "api", "/app/rest/server", "-X", "GET")
}

func TestRunChanges(t *testing.T) {
	if testBuild == nil {
		t.Skip("No test build available")
	}

	runID := fmt.Sprintf("%d", testBuild.ID)
	runCmd(t, "run", "changes", runID)
	runCmd(t, "run", "changes", runID, "--no-files")
	runCmd(t, "run", "changes", runID, "--json")
}

func TestRunTests(t *testing.T) {
	if testBuild == nil {
		t.Skip("No test build available")
	}

	runID := fmt.Sprintf("%d", testBuild.ID)
	runCmd(t, "run", "tests", runID)
	runCmd(t, "run", "tests", runID, "--failed")
	runCmd(t, "run", "tests", runID, "--json")
}

func TestRunListWithAtMe(t *testing.T) {
	client := newTestClient()
	user, err := client.GetCurrentUser()
	if err != nil {
		t.Skip("Could not get current user")
	}
	config.SetUserForServer(os.Getenv("TEAMCITY_URL"), user.Username)

	runCmd(t, "run", "list", "--user", "@me", "--limit", "5")
}

// Error handling and edge case tests

func TestInvalidIDs(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"project", []string{"project", "view", "NonExistentProject123456"}},
		{"job", []string{"job", "view", "NonExistentJob123456"}},
		{"run", []string{"run", "view", "999999999"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd := newRootCmd()
			rootCmd.SetArgs(tc.args)
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			rootCmd.Execute()
		})
	}
}

func TestListFilters(t *testing.T) {
	cases := [][]string{
		{"run", "list", "--status", "success", "--limit", "2"},
		{"run", "list", "--status", "failure", "--limit", "2"},
		{"run", "list", "--since", "24h", "--limit", "2"},
		{"project", "list", "--parent", "_Root", "--limit", "2"},
		{"job", "list", "--project", testProject, "--limit", "2"},
	}
	for _, args := range cases {
		t.Run(args[0]+"/"+args[2], func(t *testing.T) {
			runCmd(t, args...)
		})
	}
}

func TestAPICommandEdgeCases(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		t.Run(method, func(t *testing.T) {
			rootCmd := newRootCmd()
			rootCmd.SetArgs([]string{"api", "/app/rest/server", "-X", method})
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			rootCmd.Execute()
		})
	}

	t.Run("invalid_path", func(t *testing.T) {
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"api", "/app/rest/nonexistent"})
		var out bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&out)
		rootCmd.Execute()
	})
}

func TestRunViewWithBuildNumberFormat(t *testing.T) {
	if testBuild == nil {
		t.Skip("No test build available")
	}
	runCmd(t, "run", "view", fmt.Sprintf("#%s", testBuild.Number))
}

func TestOutputFormats(t *testing.T) {
	cases := [][]string{
		{"project", "list", "--json", "--limit", "1"},
		{"job", "list", "--json", "--limit", "1"},
		{"run", "list", "--json", "--limit", "1"},
		{"queue", "list", "--json"},
	}
	for _, args := range cases {
		t.Run(args[0], func(t *testing.T) {
			runCmd(t, args...)
		})
	}
}

func TestLimitFlag(t *testing.T) {
	for _, limit := range []string{"1", "5", "10", "100"} {
		t.Run(limit, func(t *testing.T) {
			runCmd(t, "run", "list", "--limit", limit)
		})
	}
}

func TestHelpCommands(t *testing.T) {
	commands := [][]string{
		{"--help"},
		{"project", "--help"},
		{"job", "--help"},
		{"run", "--help"},
		{"queue", "--help"},
		{"auth", "--help"},
		{"api", "--help"},
	}
	for _, args := range commands {
		t.Run(args[0], func(t *testing.T) {
			rootCmd := newRootCmd()
			rootCmd.SetArgs(args)
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			if err := rootCmd.Execute(); err != nil {
				t.Errorf("help failed: %v", err)
			}
			if out.Len() == 0 {
				t.Error("expected help output")
			}
		})
	}
}

func TestUnknownCommand(t *testing.T) {
	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"nonexistent"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestGlobalFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"quiet", []string{"--quiet", "project", "list", "--limit", "1"}},
		{"verbose", []string{"--verbose", "project", "list", "--limit", "1"}},
		{"no-color", []string{"--no-color", "project", "list", "--limit", "1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd := newRootCmd()
			rootCmd.SetArgs(tc.args)
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			rootCmd.Execute()
		})
	}
}
