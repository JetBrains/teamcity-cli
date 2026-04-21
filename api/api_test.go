//go:build integration || guest

// Integration tests for the TeamCity API client.
// Uses a real TeamCity server: either from TEAMCITY_URL/TEAMCITY_TOKEN env vars,
// spins up a server via testcontainers (requires Docker), or uses guest auth.
//
// Run with:
//
//	go test -tags=integration ./api/...                    # Full suite (Docker or token)
//	TEAMCITY_GUEST=1 go test -tags=guest ./api/...         # Read-only tests via guest auth
package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	client      *api.Client
	testConfig  string
	testProject string
	testBuild   *api.Build
	testEnvRef  *testEnv
)

func skipIfGuest(t *testing.T) {
	t.Helper()
	if testEnvRef != nil && testEnvRef.guestAuth {
		t.Skip("requires write access (skipped in guest mode)")
	}
}

// requireIdleAgent polls until an agent is ready and no builds are running/queued.
// It checks both server-side queue state AND agent-level build state, because the
// agent may still be processing a canceled build after the server considers it finished.
func requireIdleAgent(t *testing.T) api.Agent {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Until(deadline) > 0 {
		agents, err := client.GetAgents(api.AgentsOptions{
			Authorized: true, Connected: true, Enabled: true, Limit: 1,
		})
		if err != nil || len(agents.Agents) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}
		running, _ := client.GetBuilds(t.Context(), api.BuildsOptions{State: "running", Limit: 10})
		queued, _ := client.GetBuildQueue(api.QueueOptions{Limit: 10})
		if running != nil {
			for _, b := range running.Builds {
				_ = client.CancelBuild(fmt.Sprintf("%d", b.ID), "test cleanup")
			}
		}
		if queued != nil {
			for _, b := range queued.Builds {
				_ = client.CancelBuild(fmt.Sprintf("%d", b.ID), "test cleanup")
			}
		}
		if (running != nil && running.Count > 0) || (queued != nil && queued.Count > 0) {
			time.Sleep(2 * time.Second)
			continue
		}
		// Server says idle — now verify the agent itself isn't still processing.
		// GetAgent returns the full agent with build field populated.
		detail, err := client.GetAgent(agents.Agents[0].ID)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if detail.Build != nil {
			t.Logf("requireIdleAgent: agent %d still has build %d, waiting...", detail.ID, detail.Build.ID)
			time.Sleep(2 * time.Second)
			continue
		}
		return *detail
	}
	t.Fatal("no idle agent available within 2m")
	return api.Agent{}
}

// cancelAndWait cancels a build, polls until it finishes, then waits for its agent to release.
func cancelAndWait(t *testing.T, buildID string) {
	t.Helper()
	_ = client.CancelBuild(buildID, "test cleanup")
	var finished *api.Build
	deadline := time.Now().Add(60 * time.Second)
	for time.Until(deadline) > 0 {
		b, err := client.GetBuild(t.Context(), buildID)
		if err != nil {
			break
		}
		if b.State == "finished" {
			finished = b
			break
		}
		_ = client.CancelBuild(buildID, "test cleanup")
		time.Sleep(time.Second)
	}
	if finished != nil && finished.Agent != nil {
		waitForAgentReleased(t, finished.Agent.ID)
	} else if testEnvRef != nil && testEnvRef.ownsContainers {
		waitForOwnedAgentsReleased(t)
	}
}

// waitForAgentReleased blocks until the given agent reports .Build == nil.
func waitForAgentReleased(t *testing.T, agentID int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Until(deadline) > 0 {
		detail, err := client.GetAgent(agentID)
		if err == nil && detail.Build == nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Logf("waitForAgentReleased: agent %d did not release within 3m", agentID)
}

// waitForOwnedAgentsReleased blocks until every agent reports .Build == nil; testcontainers-only.
func waitForOwnedAgentsReleased(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Until(deadline) > 0 {
		agents, err := client.GetAgents(api.AgentsOptions{
			Authorized: true, Connected: true, Enabled: true, Limit: 10,
		})
		if err != nil || len(agents.Agents) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}
		busy := false
		for _, a := range agents.Agents {
			detail, err := client.GetAgent(a.ID)
			if err != nil || detail.Build != nil {
				busy = true
				break
			}
		}
		if !busy {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Logf("waitForOwnedAgentsReleased: agents did not release within 3m")
}

func TestMain(m *testing.M) {
	env, err := setupTestEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not setup test environment: %v\n", err)
		os.Exit(1)
	}

	client = env.Client
	testConfig = env.ConfigID
	testProject = env.ProjectID
	testBuild = env.Build
	testEnvRef = env

	if len(env.agents) > 0 {
		if err := copyBinaryToAgent(env); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not copy binary to agent: %v\n", err)
		}
	}

	code := m.Run()
	env.Cleanup()
	os.Exit(code)
}

func TestGetCurrentUser(T *testing.T) {
	skipIfGuest(T)
	T.Parallel()

	user, err := client.GetCurrentUser()
	require.NoError(T, err)
	assert.NotEmpty(T, user.Username)
}

func TestGetProjects(T *testing.T) {
	T.Parallel()

	T.Run("basic list", func(t *testing.T) {
		t.Parallel()

		projects, err := client.GetProjects(api.ProjectsOptions{Limit: 5})
		require.NoError(t, err)
		assert.Greater(t, projects.Count, 0)
	})

	T.Run("with parent filter", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetProjects(api.ProjectsOptions{Parent: "_Root", Limit: 3})
		require.NoError(t, err)
	})
}

func TestGetProject(T *testing.T) {
	T.Parallel()

	project, err := client.GetProject(testProject)
	require.NoError(T, err)
	assert.Equal(T, testProject, project.ID)
}

func TestGetBuildTypes(T *testing.T) {
	T.Parallel()

	T.Run("with project filter", func(t *testing.T) {
		t.Parallel()

		configs, err := client.GetBuildTypes(api.BuildTypesOptions{Project: testProject, Limit: 10})
		require.NoError(t, err)
		assert.Greater(t, configs.Count, 0)
	})

	T.Run("without project filter", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetBuildTypes(api.BuildTypesOptions{Limit: 5})
		require.NoError(t, err)
	})
}

func TestGetBuildType(T *testing.T) {
	T.Parallel()

	config, err := client.GetBuildType(testConfig)
	require.NoError(T, err)
	assert.Equal(T, testConfig, config.ID)
}

func TestGetBuilds(T *testing.T) {
	T.Parallel()

	T.Run("basic list", func(t *testing.T) {
		t.Parallel()

		builds, err := client.GetBuilds(t.Context(), api.BuildsOptions{BuildTypeID: testConfig, Limit: 5})
		require.NoError(t, err)
		t.Logf("Found %d builds", builds.Count)
	})

	T.Run("with filters", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetBuilds(t.Context(), api.BuildsOptions{
			BuildTypeID: testConfig,
			Status:      "success",
			State:       "finished",
			Branch:      "default:any",
			Limit:       3,
		})
		require.NoError(t, err)
	})

	T.Run("by project", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetBuilds(t.Context(), api.BuildsOptions{Project: testProject, Limit: 3})
		require.NoError(t, err)
	})
}

func TestResolveBuildID_Integration(T *testing.T) {
	// Note: passthrough and error cases are covered in unit tests (client_test.go)
	// This integration test only covers actual server-resolved cases

	T.Run("hash number resolution", func(t *testing.T) {
		if testBuild == nil {
			t.Skip("no test build available")
		}

		ref := fmt.Sprintf("#%s", testBuild.Number)
		resolvedID, err := client.ResolveBuildID(t.Context(), ref)
		require.NoError(t, err)
		assert.NotEmpty(t, resolvedID)

		build, err := client.GetBuild(t.Context(), resolvedID)
		require.NoError(t, err)
		assert.Equal(t, testBuild.Number, build.Number, "resolved build should have the requested number")
	})

	T.Run("GetBuild with hash format", func(t *testing.T) {
		if testBuild == nil {
			t.Skip("no test build available")
		}

		ref := fmt.Sprintf("#%s", testBuild.Number)
		build, err := client.GetBuild(t.Context(), ref)
		require.NoError(t, err)
		assert.Equal(t, testBuild.Number, build.Number, "returned build should have the requested number")
	})
}

func TestRunBuildAndCancel(T *testing.T) {
	skipIfGuest(T)
	requireIdleAgent(T)
	// Run with various options
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{
		Comment:      "Integration test build",
		Tags:         []string{"test", "ci"},
		Params:       map[string]string{"test.param": "value"},
		CleanSources: true,
	})
	require.NoError(T, err)
	buildID := fmt.Sprintf("%d", build.ID)
	T.Logf("Started build #%d", build.ID)
	T.Cleanup(func() { cancelAndWait(T, buildID) })

	// Verify build was created
	_, err = client.GetBuild(T.Context(), buildID)
	require.NoError(T, err)
}

func TestGetBuildQueue(T *testing.T) {
	T.Parallel()

	T.Run("basic list", func(t *testing.T) {
		t.Parallel()

		queue, err := client.GetBuildQueue(api.QueueOptions{Limit: 10})
		require.NoError(t, err)
		t.Logf("Queue has %d builds", queue.Count)
	})

	T.Run("with config filter", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetBuildQueue(api.QueueOptions{BuildTypeID: testConfig, Limit: 5})
		require.NoError(t, err)
	})
}

func TestBuildConfigPauseResume(T *testing.T) {
	skipIfGuest(T)
	// Not parallel: modifies shared config state
	err := client.SetBuildTypePaused(testConfig, true)
	require.NoError(T, err)

	err = client.SetBuildTypePaused(testConfig, false)
	require.NoError(T, err)
}

func TestProjectParameters(T *testing.T) {
	skipIfGuest(T)
	// Not parallel: creates/deletes shared project parameters
	paramName := "TC_CLI_TEST_PARAM"

	// Set regular parameter
	err := client.SetProjectParameter(testProject, paramName, "test_value", false)
	require.NoError(T, err)

	// Get parameter
	param, err := client.GetProjectParameter(testProject, paramName)
	require.NoError(T, err)
	assert.Equal(T, "test_value", param.Value)

	// List parameters
	params, err := client.GetProjectParameters(testProject)
	require.NoError(T, err)
	found := false
	for _, p := range params.Property {
		if p.Name == paramName {
			found = true
			break
		}
	}
	assert.True(T, found, "parameter should be found in list")

	// Delete parameter
	err = client.DeleteProjectParameter(testProject, paramName)
	require.NoError(T, err)

	// Test secure parameter
	err = client.SetProjectParameter(testProject, paramName, "secret", true)
	require.NoError(T, err)
	client.DeleteProjectParameter(testProject, paramName)
}

func TestBuildTypeParameters(T *testing.T) {
	skipIfGuest(T)
	// Not parallel: creates/deletes shared config parameters
	paramName := "TC_CLI_CONFIG_PARAM"

	// Set parameter
	err := client.SetBuildTypeParameter(testConfig, paramName, "config_value", false)
	require.NoError(T, err)

	// Get parameter
	param, err := client.GetBuildTypeParameter(testConfig, paramName)
	require.NoError(T, err)
	assert.Equal(T, "config_value", param.Value)

	// List parameters
	params, err := client.GetBuildTypeParameters(testConfig)
	require.NoError(T, err)
	found := false
	for _, p := range params.Property {
		if p.Name == paramName {
			found = true
			break
		}
	}
	assert.True(T, found, "config parameter should be found in list")

	// Delete parameter
	err = client.DeleteBuildTypeParameter(testConfig, paramName)
	require.NoError(T, err)
}

func TestGetServer(T *testing.T) {
	T.Parallel()

	server, err := client.GetServer()
	require.NoError(T, err)
	assert.NotEmpty(T, server.Version)

	if err := client.CheckVersion(); err != nil {
		T.Logf("Version check: %v", err)
	}

	_ = client.SupportsFeature("csrf_token")
}

func TestBuildLog(T *testing.T) {
	T.Parallel()

	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)
	log, err := client.GetBuildLog(T.Context(), buildID)
	require.NoError(T, err)
	assert.NotEmpty(T, log)
}

func TestBuildPinUnpin(T *testing.T) {
	skipIfGuest(T)
	// Not parallel: modifies testBuild pin state
	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)

	err := client.PinBuild(buildID, "Test pin")
	require.NoError(T, err)

	err = client.UnpinBuild(buildID)
	require.NoError(T, err)

	err = client.PinBuild(buildID, "")
	require.NoError(T, err)
	client.UnpinBuild(buildID)
}

func TestBuildTags(T *testing.T) {
	skipIfGuest(T)
	// Not parallel: modifies testBuild tags
	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)
	testTags := []string{"test-tag-1", "test-tag-2"}

	err := client.AddBuildTags(buildID, testTags)
	require.NoError(T, err)

	tags, err := client.GetBuildTags(buildID)
	require.NoError(T, err)
	assert.GreaterOrEqual(T, len(tags.Tag), 2)

	// Cleanup
	for _, tag := range testTags {
		client.RemoveBuildTag(buildID, tag)
	}
}

func TestBuildComment(T *testing.T) {
	skipIfGuest(T)
	// Not parallel: modifies testBuild comment
	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)

	err := client.SetBuildComment(buildID, "Test comment")
	require.NoError(T, err)

	comment, err := client.GetBuildComment(buildID)
	require.NoError(T, err)
	assert.Equal(T, "Test comment", comment)

	err = client.SetBuildComment(buildID, "Updated comment")
	require.NoError(T, err)

	err = client.DeleteBuildComment(buildID)
	require.NoError(T, err)

	comment, _ = client.GetBuildComment(buildID)
	assert.Empty(T, comment)
}

func TestQueueOperations(T *testing.T) {
	skipIfGuest(T)
	requireIdleAgent(T)
	// Queue a build
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{Comment: "Queue ops test"})
	require.NoError(T, err)
	buildID := fmt.Sprintf("%d", build.ID)
	T.Cleanup(func() { cancelAndWait(T, buildID) })

	// Try to move to top (may fail if already running)
	if err := client.MoveQueuedBuildToTop(buildID); err != nil {
		T.Logf("MoveQueuedBuildToTop: %v (build may have started)", err)
	}

	// Try to get approval info (may not be configured)
	if info, err := client.GetQueuedBuildApprovalInfo(buildID); err == nil {
		T.Logf("Approval status: %s", info.Status)
	}
}

func TestRemoveFromQueue(T *testing.T) {
	skipIfGuest(T)
	requireIdleAgent(T)
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{Comment: "Queue remove test"})
	require.NoError(T, err)

	buildID := fmt.Sprintf("%d", build.ID)
	T.Cleanup(func() { cancelAndWait(T, buildID) })

	// Remove from queue (may fail if already started)
	if err := client.RemoveFromQueue(buildID); err != nil {
		T.Logf("RemoveFromQueue: %v (may have started)", err)
	}
}

func TestGetArtifacts(T *testing.T) {
	T.Parallel()

	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)
	artifacts, err := client.GetArtifacts(T.Context(), buildID, "")
	if err != nil {
		T.Logf("GetArtifacts: %v (may be empty)", err)
		return
	}
	T.Logf("Found %d artifacts", artifacts.Count)

	if artifacts.Count > 0 {
		assert.Equal(T, artifacts.Count, len(artifacts.File), "count should match file slice length")
		for _, a := range artifacts.File {
			assert.NotEmpty(T, a.Name, "artifact should have a name")
			T.Logf("  %s (%d bytes)", a.Name, a.Size)
		}
	}
}

func TestGetArtifactsSubdirectory(T *testing.T) {
	T.Parallel()

	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)

	// First get root artifacts and find a directory entry
	rootArtifacts, err := client.GetArtifacts(T.Context(), buildID, "")
	if err != nil {
		T.Skip("could not list root artifacts:", err)
	}

	var dirName string
	for _, a := range rootArtifacts.File {
		if a.Children != nil {
			dirName = a.Name
			break
		}
	}
	if dirName == "" {
		T.Skip("no subdirectories in artifacts")
	}

	// Browse into the subdirectory
	subArtifacts, err := client.GetArtifacts(T.Context(), buildID, dirName)
	if err != nil {
		T.Fatalf("GetArtifacts(%s, %q): %v", buildID, dirName, err)
	}
	T.Logf("Found %d artifacts in %s/", subArtifacts.Count, dirName)
	assert.Greater(T, subArtifacts.Count, 0, "subdirectory should have artifacts")
	for _, a := range subArtifacts.File {
		assert.NotEmpty(T, a.Name, "artifact should have a name")
		T.Logf("  %s/%s (%d bytes)", dirName, a.Name, a.Size)
	}

	// Nonexistent path should return an error
	_, err = client.GetArtifacts(T.Context(), buildID, "nonexistent_path_12345")
	assert.Error(T, err, "nonexistent path should return error")
}

func TestDownloadArtifact(T *testing.T) {
	T.Parallel()

	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)

	artifacts, err := client.GetArtifacts(T.Context(), buildID, "")
	if err != nil {
		T.Skip("could not list artifacts:", err)
	}
	if artifacts.Count == 0 {
		T.Skip("no artifacts available")
	}

	// Find the first downloadable file (not a directory)
	var artifactPath string
	for _, a := range artifacts.File {
		if a.Size > 0 {
			artifactPath = a.Name
			break
		}
	}
	if artifactPath == "" {
		T.Skip("no downloadable artifacts found")
	}

	data, err := client.DownloadArtifact(T.Context(), buildID, artifactPath)
	require.NoError(T, err)
	assert.NotEmpty(T, data, "artifact should have content")
	T.Logf("Downloaded %d bytes from %s", len(data), artifactPath)
}

func TestGetBuildChanges(T *testing.T) {
	if testBuild == nil {
		T.Skip("no test build available")
	}

	T.Run("by_id", func(t *testing.T) {
		buildID := fmt.Sprintf("%d", testBuild.ID)
		changes, err := client.GetBuildChanges(t.Context(), buildID)
		require.NoError(t, err)
		t.Logf("Build %s has %d changes", buildID, changes.Count)
	})

	T.Run("by_number", func(t *testing.T) {
		if testBuild.Number == "" {
			t.Skip("no build number")
		}
		buildRef := fmt.Sprintf("#%s", testBuild.Number)
		changes, err := client.GetBuildChanges(t.Context(), buildRef)
		if err != nil {
			t.Logf("GetBuildChanges with build number: %v", err)
			return
		}
		t.Logf("Build %s has %d changes", buildRef, changes.Count)
	})

	T.Run("not_found", func(t *testing.T) {
		_, err := client.GetBuildChanges(t.Context(), "999999999")
		assert.Error(t, err)
	})
}

func TestGetBuildTests(T *testing.T) {
	T.Parallel()

	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)

	cases := []struct {
		name       string
		failedOnly bool
		limit      int
	}{
		{"all", false, 10},
		{"failed_only", true, 10},
		{"no_limit", false, 0},
	}

	for _, tc := range cases {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tests, err := client.GetBuildTests(t.Context(), buildID, tc.failedOnly, tc.limit)
			if err != nil {
				t.Logf("GetBuildTests: %v", err)
				return
			}
			t.Logf("count=%d passed=%d failed=%d", tests.Count, tests.Passed, tests.Failed)
		})
	}
}

func TestSupportsFeature(T *testing.T) {
	T.Parallel()

	server, err := client.ServerVersion()
	require.NoError(T, err)
	T.Logf("Server version: %s (major: %d)", server.Version, server.VersionMajor)

	features := []string{"csrf_token", "pipelines", "unknown_feature"}
	for _, f := range features {
		T.Run(f, func(t *testing.T) {
			t.Parallel()

			supported := client.SupportsFeature(f)
			t.Logf("%s: %v", f, supported)
		})
	}

	assert.True(T, client.SupportsFeature("unknown_feature"))
}

func TestUploadDiffChanges(T *testing.T) {
	skipIfGuest(T)
	T.Parallel()

	patch := []byte(`--- a/test.txt
+++ b/test.txt
@@ -1 +1 @@
-hello
+hello world
`)

	changeID, err := client.UploadDiffChanges(patch, "Integration test patch")
	require.NoError(T, err)
	assert.NotEmpty(T, changeID)
	T.Logf("Uploaded change ID: %s", changeID)
}

func TestPersonalBuildWithLocalChanges(T *testing.T) {
	skipIfGuest(T)
	requireIdleAgent(T)
	patch := []byte(`--- a/test.txt
+++ b/test.txt
@@ -1 +1 @@
-hello
+hello from personal build test
`)

	changeID, err := client.UploadDiffChanges(patch, "Personal build test")
	require.NoError(T, err)
	require.NotEmpty(T, changeID)
	T.Logf("Uploaded change ID: %s", changeID)

	build, err := client.RunBuild(testConfig, api.RunBuildOptions{
		Personal:         true,
		PersonalChangeID: changeID,
		Comment:          "Personal build with local changes",
	})
	require.NoError(T, err)
	buildID := fmt.Sprintf("%d", build.ID)
	T.Logf("Started personal build #%d", build.ID)
	T.Cleanup(func() { cancelAndWait(T, buildID) })

	fetched, err := client.GetBuild(T.Context(), buildID)
	require.NoError(T, err)
	assert.True(T, fetched.Personal, "build should be marked as personal")
	T.Logf("Build personal=%v", fetched.Personal)

	if fetched.LastChanges != nil && len(fetched.LastChanges.Change) > 0 {
		T.Logf("Build has %d changes", len(fetched.LastChanges.Change))
		for _, c := range fetched.LastChanges.Change {
			T.Logf("  Change ID=%d", c.ID)
		}
		assert.Equal(T, changeID, fmt.Sprintf("%d", fetched.LastChanges.Change[0].ID), "change ID should match")
	}
}

func TestBasicAuth(T *testing.T) {
	skipIfGuest(T)
	serverURL := os.Getenv("TEAMCITY_URL")
	require.NotEmpty(T, serverURL, "TEAMCITY_URL must be set")

	T.Run("valid credentials", func(t *testing.T) {
		basicClient := api.NewClientWithBasicAuth(serverURL, "admin", "admin123")

		user, err := basicClient.GetCurrentUser()
		require.NoError(t, err)
		assert.Equal(t, "admin", user.Username)

		server, err := basicClient.GetServer()
		require.NoError(t, err)
		assert.NotEmpty(t, server.Version)
	})

	T.Run("invalid credentials", func(t *testing.T) {
		basicClient := api.NewClientWithBasicAuth(serverURL, "invalid", "wrongpassword")
		_, err := basicClient.GetCurrentUser()
		require.Error(t, err)
	})
}

// TestBuildLevelAuth verifies that the CLI correctly uses build-level credentials
// when running inside a TeamCity build. The build runs our actual CLI binary which
// uses GetBuildAuth() to read credentials from the properties file.
func TestBuildLevelAuth(T *testing.T) {
	skipIfGuest(T)
	if testEnvRef == nil || len(testEnvRef.agents) == 0 {
		T.Skip("test requires testcontainers agent")
	}
	requireIdleAgent(T)

	configID := "Sandbox_BuildAuthTest"

	// Script runs our CLI using build-level auth.
	// We set TEAMCITY_URL to the internal Docker network name because the
	// properties file contains localhost which isn't reachable from the container.
	// Setting TEAMCITY_URL (without TEAMCITY_TOKEN) makes CLI use that URL with build credentials.
	script := `set -e
which teamcity || { echo "teamcity binary not found"; exit 1; }
export TEAMCITY_URL=http://teamcity-server:8111
unset TEAMCITY_TOKEN
teamcity auth status
`

	if !client.BuildTypeExists(configID) {
		_, err := client.CreateBuildType(testProject, api.CreateBuildTypeRequest{
			ID:   configID,
			Name: "Build Auth Test",
		})
		require.NoError(T, err)

		err = client.CreateBuildStep(configID, api.BuildStep{
			Name: "Test Build Auth",
			Type: "simpleRunner",
			Properties: api.PropertyList{
				Property: []api.Property{
					{Name: "script.content", Value: script},
					{Name: "use.custom.script", Value: "true"},
				},
			},
		})
		require.NoError(T, err)
	}

	// Retry on transient agent-handoff failures (surfacing as canceled-in-log or stuck queue).
	var buildLog string
	var build *api.Build
	var waitErr error
	for attempt := range 3 {
		if attempt > 0 {
			T.Logf("Retrying (attempt %d)...", attempt+1)
			requireIdleAgent(T)
		}

		queued, err := client.RunBuild(configID, api.RunBuildOptions{})
		require.NoError(T, err)
		buildID := fmt.Sprintf("%d", queued.ID)
		T.Logf("Started build #%d", queued.ID)
		T.Cleanup(func() { cancelAndWait(T, buildID) })

		ctx, cancel := context.WithTimeout(T.Context(), 3*time.Minute)
		build, waitErr = client.WaitForBuild(ctx, buildID, api.WaitForBuildOptions{Interval: 3 * time.Second})
		cancel()
		if waitErr != nil {
			T.Logf("WaitForBuild: %v — will retry", waitErr)
			_ = client.CancelBuild(buildID, "test retry")
			continue
		}

		buildLog, err = client.GetBuildLog(T.Context(), buildID)
		require.NoError(T, err)
		T.Logf("Build log:\n%s", buildLog)

		// Transient agent failures: server cancels the build and re-queues it
		if strings.Contains(buildLog, "Could not connect to build agent") ||
			strings.Contains(buildLog, "Agent runs unknown build") {
			T.Logf("Agent transient failure, will retry...")
			continue
		}
		break
	}
	require.NoError(T, waitErr)

	assert.Contains(T, buildLog, "Build-level credentials", "CLI should use build-level auth")
	assert.Equal(T, "SUCCESS", build.Status)
}

func TestExportProjectSettings(T *testing.T) {
	skipIfGuest(T)
	T.Run("kotlin format", func(t *testing.T) {
		t.Parallel()

		data, err := client.ExportProjectSettings(testProject, "kotlin", true)
		require.NoError(t, err)
		require.NotEmpty(t, data, "should return data")

		zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err, "should be a valid ZIP file")
		require.NotEmpty(t, zipReader.File, "ZIP should contain files")

		hasSettingsKts := false
		hasPomXml := false
		for _, f := range zipReader.File {
			t.Logf("  %s (%d bytes)", f.Name, f.UncompressedSize64)
			if strings.HasSuffix(f.Name, "settings.kts") {
				hasSettingsKts = true
			}
			if strings.HasSuffix(f.Name, "pom.xml") {
				hasPomXml = true
			}
		}
		assert.True(t, hasSettingsKts, "should contain settings.kts")
		assert.True(t, hasPomXml, "should contain pom.xml")
	})

	T.Run("xml format", func(t *testing.T) {
		t.Parallel()

		data, err := client.ExportProjectSettings(testProject, "xml", true)
		require.NoError(t, err)
		require.NotEmpty(t, data, "should return data")

		zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err, "should be a valid ZIP file")
		require.NotEmpty(t, zipReader.File, "ZIP should contain files")

		hasProjectConfig := false
		for _, f := range zipReader.File {
			t.Logf("  %s (%d bytes)", f.Name, f.UncompressedSize64)
			if strings.HasSuffix(f.Name, "project-config.xml") {
				hasProjectConfig = true
			}
		}
		assert.True(t, hasProjectConfig, "should contain project-config.xml")
	})

	T.Run("relative ids disabled", func(t *testing.T) {
		t.Parallel()

		data, err := client.ExportProjectSettings(testProject, "kotlin", false)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		_, err = zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err, "should be a valid ZIP file")
	})
}

func TestPoolOperations(T *testing.T) {
	skipIfGuest(T)
	requireIdleAgent(T)
	// Not parallel - modifies pool state

	T.Run("list pools", func(t *testing.T) {
		pools, err := client.GetAgentPools(nil)
		require.NoError(t, err)
		assert.Greater(t, pools.Count, 0, "should have at least one pool")
		t.Logf("Found %d pools", pools.Count)
	})

	T.Run("get default pool", func(t *testing.T) {
		pool, err := client.GetAgentPool(0)
		require.NoError(t, err)
		assert.Equal(t, 0, pool.ID)
		assert.NotEmpty(t, pool.Name)
		t.Logf("Default pool: %s", pool.Name)
	})

	T.Run("add and remove project from pool", func(t *testing.T) {
		pool, err := client.GetAgentPool(0)
		require.NoError(t, err)

		// Safety net: always restore the project-pool association.
		t.Cleanup(func() {
			if err := client.AddProjectToPool(pool.ID, testProject); err != nil {
				t.Logf("cleanup: AddProjectToPool: %v", err)
			}
		})

		err = client.AddProjectToPool(pool.ID, testProject)
		if err != nil {
			t.Logf("AddProjectToPool: %v (may already be in pool)", err)
		}

		err = client.RemoveProjectFromPool(pool.ID, testProject)
		require.NoError(t, err)

		err = client.AddProjectToPool(pool.ID, testProject)
		require.NoError(t, err)
	})

	T.Run("move agent to pool and back", func(t *testing.T) {
		agents, err := client.GetAgents(api.AgentsOptions{})
		require.NoError(t, err)
		if len(agents.Agents) == 0 {
			t.Skip("no agents available")
		}

		agentID := agents.Agents[0].ID

		// Get the agent's current pool
		agent, err := client.GetAgent(agentID)
		require.NoError(t, err)
		originalPoolID := agent.Pool.ID

		// Always restore the agent to the original pool
		t.Cleanup(func() {
			_ = client.SetAgentPool(agentID, originalPoolID)
		})

		// Move agent to default pool (id:0) and back
		err = client.SetAgentPool(agentID, 0)
		if err != nil {
			t.Logf("SetAgentPool to default: %v", err)
			return
		}

		// Move back to original pool
		err = client.SetAgentPool(agentID, originalPoolID)
		if err != nil {
			t.Logf("SetAgentPool back: %v", err)
		}
	})
}

func TestGetAgentIncompatibleBuildTypes(T *testing.T) {
	skipIfGuest(T)
	T.Parallel()

	agents, err := client.GetAgents(api.AgentsOptions{})
	require.NoError(T, err)
	require.Greater(T, len(agents.Agents), 0)

	incompatible, err := client.GetAgentIncompatibleBuildTypes(agents.Agents[0].ID)
	require.NoError(T, err)
	T.Logf("Agent has %d incompatible build types", incompatible.Count)
}

func TestGetParameterValue(T *testing.T) {
	skipIfGuest(T)
	// Not parallel - creates and deletes a parameter
	paramName := "TC_CLI_RAW_PARAM"
	paramValue := "raw_test_value"

	// Set a parameter on the test project
	err := client.SetProjectParameter(testProject, paramName, paramValue, false)
	require.NoError(T, err)

	// Get the raw value via GetParameterValue
	path := fmt.Sprintf("/app/rest/projects/id:%s/parameters/%s/value", testProject, paramName)
	got, err := client.GetParameterValue(path)
	require.NoError(T, err)
	assert.Equal(T, paramValue, got)

	// Cleanup
	err = client.DeleteProjectParameter(testProject, paramName)
	require.NoError(T, err)
}

func TestRunBuildAdvancedOptions(T *testing.T) {
	skipIfGuest(T)
	requireIdleAgent(T)
	T.Run("rebuild dependencies and queue at top", func(t *testing.T) {
		build, err := client.RunBuild(testConfig, api.RunBuildOptions{
			Comment:             "Test rebuild deps + queue at top",
			RebuildDependencies: true,
			QueueAtTop:          true,
		})
		require.NoError(t, err)
		buildID := fmt.Sprintf("%d", build.ID)
		t.Logf("Started build #%d with rebuild deps + queue at top", build.ID)
		t.Cleanup(func() { cancelAndWait(t, buildID) })
	})

	T.Run("with agent ID", func(t *testing.T) {
		agents, err := client.GetAgents(api.AgentsOptions{})
		require.NoError(t, err)
		if len(agents.Agents) == 0 {
			t.Skip("no agents available")
		}

		agentID := agents.Agents[0].ID
		build, err := client.RunBuild(testConfig, api.RunBuildOptions{
			Comment: "Test with agent ID",
			AgentID: agentID,
		})
		require.NoError(t, err)
		buildID := fmt.Sprintf("%d", build.ID)
		t.Logf("Started build #%d on agent %d", build.ID, agentID)
		t.Cleanup(func() { cancelAndWait(t, buildID) })
	})

	T.Run("with refs branch prefix", func(t *testing.T) {
		build, err := client.RunBuild(testConfig, api.RunBuildOptions{
			Comment: "Test with refs/ branch prefix",
			Branch:  "refs/heads/main",
		})
		require.NoError(t, err)
		buildID := fmt.Sprintf("%d", build.ID)
		t.Logf("Started build #%d with refs/ branch", build.ID)
		t.Cleanup(func() { cancelAndWait(t, buildID) })
	})
}

func TestGetAgentsPoolFilter(T *testing.T) {
	skipIfGuest(T)
	T.Parallel()

	T.Run("filter by pool name", func(t *testing.T) {
		t.Parallel()

		pool, err := client.GetAgentPool(0)
		require.NoError(t, err)

		agents, err := client.GetAgents(api.AgentsOptions{Pool: pool.Name})
		require.NoError(t, err)
		t.Logf("Found %d agents in pool '%s'", agents.Count, pool.Name)
	})

	T.Run("filter by pool numeric ID", func(t *testing.T) {
		t.Parallel()

		agents, err := client.GetAgents(api.AgentsOptions{Pool: "0"})
		require.NoError(t, err)
		t.Logf("Found %d agents in pool ID 0", agents.Count)
	})
}

func TestCancelBuildNonExistent(T *testing.T) {
	skipIfGuest(T)
	T.Parallel()

	err := client.CancelBuild("999999999", "Test cancel non-existent")
	assert.Error(T, err)
}

func TestGetBuildLogEmpty(T *testing.T) {
	skipIfGuest(T)
	requireIdleAgent(T)
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{Comment: "Empty log test"})
	require.NoError(T, err)

	buildID := fmt.Sprintf("%d", build.ID)
	T.Cleanup(func() { cancelAndWait(T, buildID) })
	cancelAndWait(T, buildID)

	log, err := client.GetBuildLog(T.Context(), buildID)
	if err != nil {
		T.Logf("GetBuildLog on cancelled build: %v", err)
		return
	}
	T.Logf("Log length for cancelled build: %d", len(log))
}

func TestGetParameterValueNonExistent(T *testing.T) {
	T.Parallel()

	path := fmt.Sprintf("/app/rest/projects/id:%s/parameters/%s/value", testProject, "NON_EXISTENT_PARAM_12345")
	_, err := client.GetParameterValue(path)
	assert.Error(T, err, "should error for non-existent parameter")
}

func TestGetBuildInvalidRef(T *testing.T) {
	T.Parallel()

	_, err := client.GetBuild(T.Context(), "999999999")
	assert.Error(T, err, "should error for invalid build ID")
}

func TestRebootAgentCancelledContext(T *testing.T) {
	skipIfGuest(T)
	T.Parallel()

	agents, err := client.GetAgents(api.AgentsOptions{})
	require.NoError(T, err)
	if len(agents.Agents) == 0 {
		T.Skip("no agents available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = client.RebootAgent(ctx, agents.Agents[0].ID, false)
	assert.Error(T, err, "should error with cancelled context")
}

func TestGetBuildQueueWithFilter(T *testing.T) {
	T.Parallel()

	queue, err := client.GetBuildQueue(api.QueueOptions{BuildTypeID: testConfig, Limit: 5})
	require.NoError(T, err)
	T.Logf("Queue has %d builds for config %s", queue.Count, testConfig)
}

// TestAgentOperations exercises the full agent API.
func TestAgentOperations(T *testing.T) {
	skipIfGuest(T)
	requireIdleAgent(T)
	// Not parallel - modifies agent state

	T.Run("list agents", func(t *testing.T) {
		agents, err := client.GetAgents(api.AgentsOptions{})
		require.NoError(t, err)
		assert.Greater(t, agents.Count, 0, "should have at least one agent")
		t.Logf("Found %d agents", agents.Count)
	})

	T.Run("get agent by id", func(t *testing.T) {
		agents, err := client.GetAgents(api.AgentsOptions{})
		require.NoError(t, err)
		require.Greater(t, len(agents.Agents), 0)

		agent, err := client.GetAgent(agents.Agents[0].ID)
		require.NoError(t, err)
		assert.Equal(t, agents.Agents[0].ID, agent.ID)
		assert.NotEmpty(t, agent.Name)
		t.Logf("Agent: %s (ID: %d)", agent.Name, agent.ID)
	})

	T.Run("get agent by name", func(t *testing.T) {
		agents, err := client.GetAgents(api.AgentsOptions{})
		require.NoError(t, err)
		require.Greater(t, len(agents.Agents), 0)

		agentName := agents.Agents[0].Name
		agent, err := client.GetAgentByName(agentName)
		require.NoError(t, err)
		assert.Equal(t, agentName, agent.Name)
		t.Logf("Found agent by name: %s (ID: %d)", agent.Name, agent.ID)
	})

	T.Run("get compatible build types", func(t *testing.T) {
		agents, err := client.GetAgents(api.AgentsOptions{})
		require.NoError(t, err)
		require.Greater(t, len(agents.Agents), 0)

		buildTypes, err := client.GetAgentCompatibleBuildTypes(agents.Agents[0].ID)
		require.NoError(t, err)
		t.Logf("Agent has %d compatible build types", buildTypes.Count)
	})

	T.Run("enable and disable", func(t *testing.T) {
		agents, err := client.GetAgents(api.AgentsOptions{})
		require.NoError(t, err)
		require.Greater(t, len(agents.Agents), 0)

		agentID := agents.Agents[0].ID

		// Always re-enable the agent, even if the test fails midway.
		t.Cleanup(func() {
			_ = client.EnableAgent(agentID, true)
		})

		// Disable
		err = client.EnableAgent(agentID, false)
		require.NoError(t, err)

		agent, err := client.GetAgent(agentID)
		require.NoError(t, err)
		assert.False(t, agent.Enabled, "agent should be disabled")

		// Re-enable
		err = client.EnableAgent(agentID, true)
		require.NoError(t, err)

		readyAgent := requireIdleAgent(t)
		assert.Equal(t, agentID, readyAgent.ID)
		assert.True(t, readyAgent.Enabled, "agent should be enabled")
	})

}

func TestWaitForBuild_Integration(T *testing.T) {
	skipIfGuest(T)
	requireIdleAgent(T)

	build, err := client.RunBuild(testConfig, api.RunBuildOptions{
		Comment: "WaitForBuild integration test",
	})
	require.NoError(T, err)
	buildID := fmt.Sprintf("%d", build.ID)
	T.Logf("Queued build #%d, waiting for completion...", build.ID)

	ctx, cancel := context.WithTimeout(T.Context(), 5*time.Minute)
	defer cancel()

	var progressCalls int
	result, err := client.WaitForBuild(ctx, buildID, api.WaitForBuildOptions{
		Interval: 3 * time.Second,
		OnProgress: func(state, status string, percent int) error {
			progressCalls++
			T.Logf("  poll #%d: state=%s status=%s percent=%d", progressCalls, state, status, percent)
			return nil
		},
	})
	require.NoError(T, err)

	assert.Equal(T, "finished", result.State)
	assert.Equal(T, "SUCCESS", result.Status)
	assert.NotEmpty(T, result.Number)
	assert.NotEmpty(T, result.WebURL)
	assert.Greater(T, progressCalls, 0)
	T.Logf("Build #%d finished: status=%s number=%s (%d polls)", result.ID, result.Status, result.Number, progressCalls)
}

// TestRawRequestInvalidField verifies real error (404) is returned, not misleading 406.
func TestRawRequestInvalidField(T *testing.T) {
	T.Parallel()

	require.NotNil(T, testBuild, "need a finished build")

	endpoint := fmt.Sprintf("/app/rest/builds/id:%d/snapshot-dependencies", testBuild.ID)
	resp, err := client.RawRequest(T.Context(), "GET", endpoint, nil, nil)
	require.NoError(T, err)

	assert.NotEqual(T, 406, resp.StatusCode)
	assert.Equal(T, 404, resp.StatusCode)
	assert.Contains(T, string(resp.Body), "snapshot-dependencies")
}

// TestRequestHeadersServerSide checks the server receives and logs User-Agent and X-TeamCity-Client.
func TestRequestHeadersServerSide(T *testing.T) {
	if testEnvRef == nil || testEnvRef.server == nil {
		T.Skip("requires testcontainers")
	}

	ctx := T.Context()

	// Enable DEBUG on RequestDiagnosticProvider so it logs every request with full headers.
	_, output, err := testEnvRef.server.Exec(ctx, []string{
		"sh", "-c", `
			for f in /opt/teamcity/conf/teamcity-server-log4j.xml \
			          /opt/teamcity/conf/teamcity-server-log4j2.xml; do
				if [ -f "$f" ] && ! grep -q 'RequestDiagnosticProvider' "$f"; then
					sed -i 's|</Loggers>|  <Logger name="jetbrains.buildServer.diagnostic.web.RequestDiagnosticProvider" level="DEBUG"/>\n  </Loggers>|' "$f"
					sed -i 's|</log4j:configuration>|  <category name="jetbrains.buildServer.diagnostic.web.RequestDiagnosticProvider"><priority value="DEBUG"/></category>\n</log4j:configuration>|' "$f"
				fi
			done
		`,
	})
	require.NoError(T, err)
	_, _ = io.ReadAll(output)

	adminClient := api.NewClient(testEnvRef.URL, testEnvRef.Token)
	_, _ = adminClient.RawRequest(T.Context(), "POST",
		"/app/rest/server/internals/diagnostics/threadDumps/reload", nil, nil)
	time.Sleep(10 * time.Second)

	c := api.NewClient(testEnvRef.URL, testEnvRef.Token,
		api.WithVersion("42.0.0-test"),
		api.WithCommandName("integration-test"),
	)

	for range 3 {
		_, _ = c.GetServer()
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(3 * time.Second)

	_, output, err = testEnvRef.server.Exec(ctx, []string{
		"grep", "-r", "-a", "42.0.0-test", "/opt/teamcity/logs/",
	})
	require.NoError(T, err)
	logContent, _ := io.ReadAll(output)
	logs := string(logContent)

	found := strings.Contains(logs, "42.0.0-test")
	if !found {
		reader, err := testEnvRef.server.Logs(ctx)
		require.NoError(T, err)
		defer reader.Close()
		allLogs, _ := io.ReadAll(reader)
		logs = string(allLogs)
		found = strings.Contains(logs, "42.0.0-test")
	}

	require.True(T, found, "server logs must contain 'teamcity-cli/42.0.0-test'")
	assert.Contains(T, logs, "user-agent")
	assert.Contains(T, logs, "client: teamcity-cli/42.0.0-test")
}
