package api_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/tiulpin/teamcity-cli/internal/api"
)

var (
	client      *api.Client
	testConfig  string
	testProject string
)

func TestMain(m *testing.M) {
	err := godotenv.Load("../../.env")
	if err != nil {
		return
	}

	url := os.Getenv("TEAMCITY_URL")
	token := os.Getenv("TEAMCITY_TOKEN")
	testConfig = os.Getenv("TEAMCITY_TEST_CONFIG")
	testProject = os.Getenv("TEAMCITY_TEST_PROJECT")

	if url == "" || token == "" {
		println("Skipping integration tests: TEAMCITY_URL or TEAMCITY_TOKEN not set")
		os.Exit(0)
	}

	client = api.NewClient(url, token)
	os.Exit(m.Run())
}

func TestGetCurrentUser(t *testing.T) {
	user, err := client.GetCurrentUser()
	if err != nil {
		t.Fatalf("GetCurrentUser failed: %v", err)
	}
	if user.Username == "" {
		t.Error("Expected username to be set")
	}
}

func TestGetProjects(t *testing.T) {
	// Basic list
	projects, err := client.GetProjects(api.ProjectsOptions{Limit: 5})
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}
	if projects.Count == 0 {
		t.Error("Expected at least one project")
	}

	// With parent filter
	_, err = client.GetProjects(api.ProjectsOptions{Parent: "_Root", Limit: 3})
	if err != nil {
		t.Fatalf("GetProjects with parent failed: %v", err)
	}
}

func TestGetProject(t *testing.T) {
	project, err := client.GetProject(testProject)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if project.ID != testProject {
		t.Errorf("Expected project ID %s, got %s", testProject, project.ID)
	}
}

func TestGetBuildTypes(t *testing.T) {
	// With project filter
	configs, err := client.GetBuildTypes(api.BuildTypesOptions{Project: testProject, Limit: 10})
	if err != nil {
		t.Fatalf("GetBuildTypes failed: %v", err)
	}
	if configs.Count == 0 {
		t.Error("Expected at least one build config")
	}

	// Without project filter
	_, err = client.GetBuildTypes(api.BuildTypesOptions{Limit: 5})
	if err != nil {
		t.Fatalf("GetBuildTypes without project failed: %v", err)
	}
}

func TestGetBuildType(t *testing.T) {
	config, err := client.GetBuildType(testConfig)
	if err != nil {
		t.Fatalf("GetBuildType failed: %v", err)
	}
	if config.ID != testConfig {
		t.Errorf("Expected config ID %s, got %s", testConfig, config.ID)
	}
}

func TestGetBuilds(t *testing.T) {
	// Basic list
	builds, err := client.GetBuilds(api.BuildsOptions{BuildTypeID: testConfig, Limit: 5})
	if err != nil {
		t.Fatalf("GetBuilds failed: %v", err)
	}
	t.Logf("Found %d builds", builds.Count)

	// With filters (status, state, branch, project)
	_, err = client.GetBuilds(api.BuildsOptions{
		BuildTypeID: testConfig,
		Status:      "success",
		State:       "finished",
		Branch:      "default:any",
		Limit:       3,
	})
	if err != nil {
		t.Fatalf("GetBuilds with filters failed: %v", err)
	}

	// By project
	_, err = client.GetBuilds(api.BuildsOptions{Project: testProject, Limit: 3})
	if err != nil {
		t.Fatalf("GetBuilds by project failed: %v", err)
	}
}

func TestResolveBuildID(t *testing.T) {
	// Test plain ID passthrough
	id, err := client.ResolveBuildID("12345")
	if err != nil {
		t.Fatalf("ResolveBuildID plain ID failed: %v", err)
	}
	if id != "12345" {
		t.Errorf("Expected plain ID to pass through, got %s", id)
	}

	// Test #number resolution
	builds, err := client.GetBuilds(api.BuildsOptions{BuildTypeID: testConfig, Limit: 1})
	if err != nil {
		t.Fatalf("GetBuilds failed: %v", err)
	}
	if builds.Count == 0 {
		t.Skip("No builds to test #number resolution")
	}

	build := builds.Builds[0]
	ref := fmt.Sprintf("#%s", build.Number)
	resolvedID, err := client.ResolveBuildID(ref)
	if err != nil {
		t.Fatalf("ResolveBuildID #number failed: %v", err)
	}
	expectedID := fmt.Sprintf("%d", build.ID)
	if resolvedID != expectedID {
		t.Errorf("Expected resolved ID %s, got %s", expectedID, resolvedID)
	}

	// Test invalid #number
	_, err = client.ResolveBuildID("#999999999")
	if err == nil {
		t.Error("Expected error for invalid build number")
	}

	// Test GetBuild with #number format
	fetchedBuild, err := client.GetBuild(ref)
	if err != nil {
		t.Fatalf("GetBuild with #number failed: %v", err)
	}
	if fetchedBuild.ID != build.ID {
		t.Errorf("GetBuild #number returned wrong build: expected %d, got %d", build.ID, fetchedBuild.ID)
	}
}

func TestRunBuildAndCancel(t *testing.T) {
	// Run with various options
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{
		Comment:      "Integration test build",
		Tags:         []string{"test", "ci"},
		Params:       map[string]string{"test.param": "value"},
		CleanSources: true,
	})
	if err != nil {
		t.Fatalf("RunBuild failed: %v", err)
	}
	t.Logf("Started build #%d", build.ID)

	// Verify build was created
	fetched, err := client.GetBuild(fmt.Sprintf("%d", build.ID))
	if err != nil {
		t.Fatalf("GetBuild failed: %v", err)
	}

	// Cancel if still in queue/running
	if fetched.State == "queued" || fetched.State == "running" {
		err = client.CancelBuild(fmt.Sprintf("%d", build.ID), "Integration test cleanup")
		if err != nil {
			t.Logf("CancelBuild warning (may have finished): %v", err)
		}
	}
}

func TestGetBuildQueue(t *testing.T) {
	// Basic queue list
	queue, err := client.GetBuildQueue(api.QueueOptions{Limit: 10})
	if err != nil {
		t.Fatalf("GetBuildQueue failed: %v", err)
	}
	t.Logf("Queue has %d builds", queue.Count)

	// With config filter
	_, err = client.GetBuildQueue(api.QueueOptions{BuildTypeID: testConfig, Limit: 5})
	if err != nil {
		t.Fatalf("GetBuildQueue with config failed: %v", err)
	}
}

func TestBuildConfigPauseResume(t *testing.T) {
	if err := client.PauseBuildType(testConfig); err != nil {
		t.Fatalf("PauseBuildType failed: %v", err)
	}

	if err := client.ResumeBuildType(testConfig); err != nil {
		t.Fatalf("ResumeBuildType failed: %v", err)
	}
}

func TestProjectParameters(t *testing.T) {
	paramName := "TC_CLI_TEST_PARAM"

	// Set regular parameter
	if err := client.SetProjectParameter(testProject, paramName, "test_value", false); err != nil {
		t.Fatalf("SetProjectParameter failed: %v", err)
	}

	// Get parameter
	param, err := client.GetProjectParameter(testProject, paramName)
	if err != nil {
		t.Fatalf("GetProjectParameter failed: %v", err)
	}
	if param.Value != "test_value" {
		t.Errorf("Expected value test_value, got %s", param.Value)
	}

	// List parameters
	params, err := client.GetProjectParameters(testProject)
	if err != nil {
		t.Fatalf("GetProjectParameters failed: %v", err)
	}
	found := false
	for _, p := range params.Property {
		if p.Name == paramName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Parameter not found in list")
	}

	// Delete parameter
	if err := client.DeleteProjectParameter(testProject, paramName); err != nil {
		t.Fatalf("DeleteProjectParameter failed: %v", err)
	}

	// Test secure parameter
	if err := client.SetProjectParameter(testProject, paramName, "secret", true); err != nil {
		t.Fatalf("SetProjectParameter (secure) failed: %v", err)
	}
	client.DeleteProjectParameter(testProject, paramName)
}

func TestBuildTypeParameters(t *testing.T) {
	paramName := "TC_CLI_CONFIG_PARAM"

	// Set parameter
	if err := client.SetBuildTypeParameter(testConfig, paramName, "config_value", false); err != nil {
		t.Fatalf("SetBuildTypeParameter failed: %v", err)
	}

	// Get parameter
	param, err := client.GetBuildTypeParameter(testConfig, paramName)
	if err != nil {
		t.Fatalf("GetBuildTypeParameter failed: %v", err)
	}
	if param.Value != "config_value" {
		t.Errorf("Expected value config_value, got %s", param.Value)
	}

	// List parameters
	params, err := client.GetBuildTypeParameters(testConfig)
	if err != nil {
		t.Fatalf("GetBuildTypeParameters failed: %v", err)
	}
	found := false
	for _, p := range params.Property {
		if p.Name == paramName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Config parameter not found in list")
	}

	// Delete parameter
	if err := client.DeleteBuildTypeParameter(testConfig, paramName); err != nil {
		t.Fatalf("DeleteBuildTypeParameter failed: %v", err)
	}
}

func TestGetServer(t *testing.T) {
	server, err := client.GetServer()
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if server.Version == "" {
		t.Error("Expected server version to be set")
	}

	// Test version check
	if err := client.CheckVersion(); err != nil {
		t.Logf("Version check: %v", err)
	}

	// Test feature support
	_ = client.SupportsFeature("csrf_token")
}

func TestBuildLog(t *testing.T) {
	builds, err := client.GetBuilds(api.BuildsOptions{
		BuildTypeID: testConfig,
		Status:      "success",
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("GetBuilds failed: %v", err)
	}
	if builds.Count == 0 {
		t.Skip("No successful builds to test logs")
	}

	buildID := fmt.Sprintf("%d", builds.Builds[0].ID)
	log, err := client.GetBuildLog(buildID)
	if err != nil {
		t.Fatalf("GetBuildLog failed: %v", err)
	}
	if len(log) == 0 {
		t.Error("Expected non-empty log")
	}
}

func TestBuildPinUnpin(t *testing.T) {
	builds, err := client.GetBuilds(api.BuildsOptions{
		BuildTypeID: testConfig,
		Status:      "success",
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("GetBuilds failed: %v", err)
	}
	if builds.Count == 0 {
		t.Skip("No successful builds to test pinning")
	}

	buildID := fmt.Sprintf("%d", builds.Builds[0].ID)

	// Pin with comment
	if err := client.PinBuild(buildID, "Test pin"); err != nil {
		t.Fatalf("PinBuild failed: %v", err)
	}

	// Unpin
	if err := client.UnpinBuild(buildID); err != nil {
		t.Fatalf("UnpinBuild failed: %v", err)
	}

	// Pin without comment (uses default)
	if err := client.PinBuild(buildID, ""); err != nil {
		t.Fatalf("PinBuild without comment failed: %v", err)
	}
	client.UnpinBuild(buildID)
}

func TestBuildTags(t *testing.T) {
	builds, err := client.GetBuilds(api.BuildsOptions{
		BuildTypeID: testConfig,
		Status:      "success",
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("GetBuilds failed: %v", err)
	}
	if builds.Count == 0 {
		t.Skip("No successful builds to test tagging")
	}

	buildID := fmt.Sprintf("%d", builds.Builds[0].ID)
	testTags := []string{"test-tag-1", "test-tag-2"}

	// Add tags
	if err := client.AddBuildTags(buildID, testTags); err != nil {
		t.Fatalf("AddBuildTags failed: %v", err)
	}

	// Get tags
	tags, err := client.GetBuildTags(buildID)
	if err != nil {
		t.Fatalf("GetBuildTags failed: %v", err)
	}
	if len(tags.Tag) < 2 {
		t.Error("Expected at least 2 tags")
	}

	// Remove tags
	for _, tag := range testTags {
		client.RemoveBuildTag(buildID, tag)
	}
}

func TestBuildComment(t *testing.T) {
	builds, err := client.GetBuilds(api.BuildsOptions{
		BuildTypeID: testConfig,
		Status:      "success",
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("GetBuilds failed: %v", err)
	}
	if builds.Count == 0 {
		t.Skip("No successful builds to test comments")
	}

	buildID := fmt.Sprintf("%d", builds.Builds[0].ID)

	// Set comment
	if err := client.SetBuildComment(buildID, "Test comment"); err != nil {
		t.Fatalf("SetBuildComment failed: %v", err)
	}

	// Get comment
	comment, err := client.GetBuildComment(buildID)
	if err != nil {
		t.Fatalf("GetBuildComment failed: %v", err)
	}
	if comment != "Test comment" {
		t.Errorf("Expected 'Test comment', got %q", comment)
	}

	// Update comment
	if err := client.SetBuildComment(buildID, "Updated comment"); err != nil {
		t.Fatalf("SetBuildComment (update) failed: %v", err)
	}

	// Delete comment
	if err := client.DeleteBuildComment(buildID); err != nil {
		t.Fatalf("DeleteBuildComment failed: %v", err)
	}

	// Verify deletion
	comment, _ = client.GetBuildComment(buildID)
	if comment != "" {
		t.Errorf("Expected empty comment after deletion, got %q", comment)
	}
}

func TestQueueOperations(t *testing.T) {
	// Queue a build
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{Comment: "Queue ops test"})
	if err != nil {
		t.Fatalf("RunBuild failed: %v", err)
	}
	buildID := fmt.Sprintf("%d", build.ID)

	// Try to move to top (may fail if already running)
	if err := client.MoveQueuedBuildToTop(buildID); err != nil {
		t.Logf("MoveQueuedBuildToTop: %v (build may have started)", err)
	}

	// Try to get approval info (may not be configured)
	if info, err := client.GetQueuedBuildApprovalInfo(buildID); err == nil {
		t.Logf("Approval status: %s", info.Status)
	}

	// Cleanup
	client.CancelBuild(buildID, "Test cleanup")
}

func TestRemoveFromQueue(t *testing.T) {
	build, err := client.RunBuild(testConfig, api.RunBuildOptions{Comment: "Queue remove test"})
	if err != nil {
		t.Fatalf("RunBuild failed: %v", err)
	}

	// Remove from queue (may fail if already started)
	if err := client.RemoveFromQueue(fmt.Sprintf("%d", build.ID)); err != nil {
		t.Logf("RemoveFromQueue: %v (may have started)", err)
		client.CancelBuild(fmt.Sprintf("%d", build.ID), "Test cleanup")
	}
}

func TestGetArtifacts(t *testing.T) {
	builds, err := client.GetBuilds(api.BuildsOptions{
		BuildTypeID: testConfig,
		Status:      "success",
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("GetBuilds failed: %v", err)
	}
	if builds.Count == 0 {
		t.Skip("No successful builds to test artifacts")
	}

	buildID := fmt.Sprintf("%d", builds.Builds[0].ID)
	artifacts, err := client.GetArtifacts(buildID)
	if err != nil {
		t.Logf("GetArtifacts: %v (may be empty)", err)
		return
	}
	t.Logf("Found %d artifacts", artifacts.Count)
}

func TestParseTeamCityTime(t *testing.T) {
	parsed, err := api.ParseTeamCityTime("20250710T080607+0000")
	if err != nil {
		t.Fatalf("ParseTeamCityTime failed: %v", err)
	}
	if parsed.Year() != 2025 || parsed.Month() != 7 {
		t.Errorf("Unexpected parsed time: %v", parsed)
	}
}
