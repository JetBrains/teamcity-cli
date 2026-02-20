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

	if env.agent != nil {
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
	skipIfGuest(T)
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
	log, err := client.GetBuildLog(buildID)
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
	skipIfGuest(T)
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
	artifacts, err := client.GetArtifacts(buildID, "")
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
	rootArtifacts, err := client.GetArtifacts(buildID, "")
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
	subArtifacts, err := client.GetArtifacts(buildID, dirName)
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
	_, err = client.GetArtifacts(buildID, "nonexistent_path_12345")
	assert.Error(T, err, "nonexistent path should return error")
}

func TestDownloadArtifact(T *testing.T) {
	T.Parallel()

	if testBuild == nil {
		T.Skip("no test build available")
	}

	buildID := fmt.Sprintf("%d", testBuild.ID)

	artifacts, err := client.GetArtifacts(buildID, "")
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
	if testEnvRef == nil || testEnvRef.agent == nil {
		T.Skip("test requires testcontainers agent")
	}

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

	build, err := client.RunBuild(configID, api.RunBuildOptions{})
	require.NoError(T, err)
	T.Logf("Started build #%d", build.ID)

	buildID := fmt.Sprintf("%d", build.ID)
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		build, err = client.GetBuild(buildID)
		require.NoError(T, err)
		if build.State == "finished" {
			break
		}
		time.Sleep(3 * time.Second)
	}

	require.Equal(T, "finished", build.State)

	buildLog, err := client.GetBuildLog(buildID)
	require.NoError(T, err)
	T.Logf("Build log:\n%s", buildLog)

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
		// Get the default pool first
		pool, err := client.GetAgentPool(0)
		require.NoError(t, err)

		// Add the test project to the default pool
		err = client.AddProjectToPool(pool.ID, testProject)
		if err != nil {
			t.Logf("AddProjectToPool: %v (project may already be in pool)", err)
		} else {
			// Only try to remove if adding succeeded
			err = client.RemoveProjectFromPool(pool.ID, testProject)
			if err != nil {
				t.Logf("RemoveProjectFromPool: %v", err)
			}
		}
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
	T.Run("rebuild dependencies and queue at top", func(t *testing.T) {
		build, err := client.RunBuild(testConfig, api.RunBuildOptions{
			Comment:             "Test rebuild deps + queue at top",
			RebuildDependencies: true,
			QueueAtTop:          true,
		})
		require.NoError(t, err)
		t.Logf("Started build #%d with rebuild deps + queue at top", build.ID)

		client.CancelBuild(fmt.Sprintf("%d", build.ID), "Test cleanup")
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
		t.Logf("Started build #%d on agent %d", build.ID, agentID)

		client.CancelBuild(fmt.Sprintf("%d", build.ID), "Test cleanup")
	})

	T.Run("with refs branch prefix", func(t *testing.T) {
		build, err := client.RunBuild(testConfig, api.RunBuildOptions{
			Comment: "Test with refs/ branch prefix",
			Branch:  "refs/heads/main",
		})
		require.NoError(t, err)
		t.Logf("Started build #%d with refs/ branch", build.ID)

		client.CancelBuild(fmt.Sprintf("%d", build.ID), "Test cleanup")
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
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{Comment: "Empty log test"})
	require.NoError(T, err)

	buildID := fmt.Sprintf("%d", build.ID)
	client.CancelBuild(buildID, "Empty log test cleanup")

	log, err := client.GetBuildLog(buildID)
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

	_, err := client.GetBuild("999999999")
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

// TestZAgentOperations runs last (Z prefix) since reboot affects agent availability.
// This test exercises the full agent API, including operations that modify the agent state.
func TestZAgentOperations(T *testing.T) {
	skipIfGuest(T)
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

		// Disable
		err = client.EnableAgent(agentID, false)
		require.NoError(t, err)

		agent, err := client.GetAgent(agentID)
		require.NoError(t, err)
		assert.False(t, agent.Enabled, "agent should be disabled")

		// Re-enable
		err = client.EnableAgent(agentID, true)
		require.NoError(t, err)

		agent, err = client.GetAgent(agentID)
		require.NoError(t, err)
		assert.True(t, agent.Enabled, "agent should be enabled")
	})

	T.Run("reboot agent", func(t *testing.T) {
		agents, err := client.GetAgents(api.AgentsOptions{})
		require.NoError(t, err)
		require.Greater(t, len(agents.Agents), 0)

		agentID := agents.Agents[0].ID
		t.Logf("Requesting reboot for agent ID %d", agentID)

		err = client.RebootAgent(context.Background(), agentID, true)
		require.NoError(t, err)
		t.Log("Reboot scheduled (after build completes)")
	})
}
