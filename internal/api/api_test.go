//go:build integration

// Integration tests for the TeamCity API client.
// Uses a real TeamCity server: either from TEAMCITY_URL/TEAMCITY_TOKEN env vars,
// or spins up a server via testcontainers (requires Docker).
//
// Run with: go test -tags=integration ./internal/api/...
package api_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	client      *api.Client
	testConfig  string
	testProject string
	testBuild   *api.Build
)

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

	code := m.Run()
	env.Cleanup()
	os.Exit(code)
}

func TestGetCurrentUser(T *testing.T) {
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

		builds, err := client.GetBuilds(api.BuildsOptions{BuildTypeID: testConfig, Limit: 5})
		require.NoError(t, err)
		t.Logf("Found %d builds", builds.Count)
	})

	T.Run("with filters", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetBuilds(api.BuildsOptions{
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

		_, err := client.GetBuilds(api.BuildsOptions{Project: testProject, Limit: 3})
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
		resolvedID, err := client.ResolveBuildID(ref)
		require.NoError(t, err)
		wantID := fmt.Sprintf("%d", testBuild.ID)
		assert.Equal(t, wantID, resolvedID)
	})

	T.Run("GetBuild with hash format", func(t *testing.T) {
		if testBuild == nil {
			t.Skip("no test build available")
		}

		ref := fmt.Sprintf("#%s", testBuild.Number)
		build, err := client.GetBuild(ref)
		require.NoError(t, err)
		assert.Equal(t, testBuild.ID, build.ID)
	})
}

func TestRunBuildAndCancel(T *testing.T) {
	// Run with various options
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{
		Comment:      "Integration test build",
		Tags:         []string{"test", "ci"},
		Params:       map[string]string{"test.param": "value"},
		CleanSources: true,
	})
	require.NoError(T, err)
	T.Logf("Started build #%d", build.ID)

	// Verify build was created
	fetched, err := client.GetBuild(fmt.Sprintf("%d", build.ID))
	require.NoError(T, err)

	// Cancel if still in queue/running
	if fetched.State == "queued" || fetched.State == "running" {
		err = client.CancelBuild(fmt.Sprintf("%d", build.ID), "Integration test cleanup")
		if err != nil {
			T.Logf("CancelBuild warning (may have finished): %v", err)
		}
	}
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
	// Not parallel: modifies shared config state
	err := client.SetBuildTypePaused(testConfig, true)
	require.NoError(T, err)

	err = client.SetBuildTypePaused(testConfig, false)
	require.NoError(T, err)
}

func TestProjectParameters(T *testing.T) {
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
	log, err := client.GetBuildLog(buildID)
	require.NoError(T, err)
	assert.NotEmpty(T, log)
}

func TestBuildPinUnpin(T *testing.T) {
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
	// Queue a build
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{Comment: "Queue ops test"})
	require.NoError(T, err)
	buildID := fmt.Sprintf("%d", build.ID)

	// Try to move to top (may fail if already running)
	if err := client.MoveQueuedBuildToTop(buildID); err != nil {
		T.Logf("MoveQueuedBuildToTop: %v (build may have started)", err)
	}

	// Try to get approval info (may not be configured)
	if info, err := client.GetQueuedBuildApprovalInfo(buildID); err == nil {
		T.Logf("Approval status: %s", info.Status)
	}

	// Cleanup
	client.CancelBuild(buildID, "Test cleanup")
}

func TestRemoveFromQueue(T *testing.T) {
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{Comment: "Queue remove test"})
	require.NoError(T, err)

	// Remove from queue (may fail if already started)
	if err := client.RemoveFromQueue(fmt.Sprintf("%d", build.ID)); err != nil {
		T.Logf("RemoveFromQueue: %v (may have started)", err)
		client.CancelBuild(fmt.Sprintf("%d", build.ID), "Test cleanup")
	}
}

func TestGetArtifacts(T *testing.T) {
	T.Parallel()

	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)
	artifacts, err := client.GetArtifacts(buildID)
	if err != nil {
		T.Logf("GetArtifacts: %v (may be empty)", err)
		return
	}
	T.Logf("Found %d artifacts", artifacts.Count)
}

func TestDownloadArtifact(T *testing.T) {
	T.Parallel()

	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)

	artifacts, err := client.GetArtifacts(buildID)
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

	data, err := client.DownloadArtifact(buildID, artifactPath)
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
		changes, err := client.GetBuildChanges(buildID)
		require.NoError(t, err)
		t.Logf("Build %s has %d changes", buildID, changes.Count)
	})

	T.Run("by_number", func(t *testing.T) {
		if testBuild.Number == "" {
			t.Skip("no build number")
		}
		buildRef := fmt.Sprintf("#%s", testBuild.Number)
		changes, err := client.GetBuildChanges(buildRef)
		if err != nil {
			t.Logf("GetBuildChanges with build number: %v", err)
			return
		}
		t.Logf("Build %s has %d changes", buildRef, changes.Count)
	})

	T.Run("not_found", func(t *testing.T) {
		_, err := client.GetBuildChanges("999999999")
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

			tests, err := client.GetBuildTests(buildID, tc.failedOnly, tc.limit)
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
	T.Logf("Started personal build #%d", build.ID)

	fetched, err := client.GetBuild(fmt.Sprintf("%d", build.ID))
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

	if fetched.State == "queued" || fetched.State == "running" {
		err = client.CancelBuild(fmt.Sprintf("%d", build.ID), "Integration test cleanup")
		if err != nil {
			T.Logf("CancelBuild warning (may have finished): %v", err)
		}
	}
}

func TestBasicAuthClient(T *testing.T) {
	// This test verifies that NewClientWithBasicAuth works correctly.
	// Build-auth uses basic authentication under the hood.
	T.Parallel()

	// Use the existing test environment URL and admin credentials
	serverURL := os.Getenv("TEAMCITY_URL")
	if serverURL == "" {
		T.Skip("TEAMCITY_URL not set")
	}

	// Create a client using basic auth with admin credentials
	// (the testenv_test.go creates an admin user with password "admin123")
	basicClient := api.NewClientWithBasicAuth(serverURL, "admin", "admin123")

	// Test that the client can authenticate and make API calls
	user, err := basicClient.GetCurrentUser()
	if err != nil {
		// If admin user doesn't exist with these credentials, just skip
		T.Skipf("Basic auth test skipped (admin credentials may differ): %v", err)
	}

	assert.Equal(T, "admin", user.Username)
	T.Logf("Basic auth test passed for user: %s", user.Username)
}
