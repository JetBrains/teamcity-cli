package cmdtest

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
)

// SetupMockClient creates a test server with all common API endpoints pre-registered.
func SetupMockClient(t *testing.T) *TestServer {
	t.Helper()
	ts := NewTestServer(t)

	// Server
	ts.Handle("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.Server{
			Version:      " (build 197398)",
			VersionMajor: 2025,
			VersionMinor: 7,
			BuildNumber:  "197398",
			WebURL:       ts.URL,
		})
	})

	ts.Handle("HEAD /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("GET /app/rest/server/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		_, _ = w.Write([]byte("2025.7 (build 197398)"))
	})

	ts.Handle("GET /app/rest/anything", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, map[string]string{"path": "anything"})
	})

	// Users
	ts.Handle("GET /app/rest/users/current", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.User{ID: 1, Username: "admin", Name: "Administrator"})
	})

	ts.Handle("GET /app/rest/users/username:", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.User{ID: 1, Username: "testuser", Name: "Test User"})
	})

	// Projects list
	ts.Handle("GET /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "NonExistentProject123456") {
			Error(w, http.StatusNotFound, "No project found by locator 'id:NonExistentProject123456'")
			return
		}
		JSON(w, api.ProjectList{
			Count: 2,
			Projects: []api.Project{
				{ID: "_Root", Name: "Root project", ParentProjectID: ""},
				{ID: "TestProject", Name: "Test Project", ParentProjectID: "_Root"},
			},
		})
	})

	ts.Handle("POST /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		var req api.CreateProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, err.Error())
			return
		}
		id := req.ID
		if id == "" {
			id = req.Name
		}
		parent := "_Root"
		if req.ParentProject != nil {
			parent = req.ParentProject.ID
		}
		JSON(w, api.Project{
			ID:              id,
			Name:            req.Name,
			ParentProjectID: parent,
			WebURL:          ts.URL + "/project.html?projectId=" + id,
		})
	})

	// Projects by ID
	ts.Handle("GET /app/rest/projects/id:", func(w http.ResponseWriter, r *http.Request) {
		id := ExtractID(r.URL.Path, "id:")
		if id == "NonExistentProject123456" {
			Error(w, http.StatusNotFound, "No project found by locator 'id:NonExistentProject123456'")
			return
		}

		if strings.Contains(r.URL.Path, "/parameters/") {
			JSON(w, api.Parameter{Name: "param1", Value: "value1"})
			return
		}
		if strings.Contains(r.URL.Path, "/parameters") {
			JSON(w, api.ParameterList{
				Count:    1,
				Property: []api.Parameter{{Name: "param1", Value: "value1"}},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/secure/values") {
			Text(w, "secret-value")
			return
		}

		JSON(w, api.Project{
			ID:              id,
			Name:            "Test Project",
			ParentProjectID: "_Root",
			WebURL:          ts.URL + "/project.html?projectId=" + id,
		})
	})

	ts.Handle("POST /app/rest/projects/id:", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/secure/tokens") {
			Text(w, "credentialsJSON:abc123")
			return
		}
		if strings.Contains(r.URL.Path, "/parameters") {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("POST /app/rest/projects/TestProject", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/secure/tokens") {
			Text(w, "credentialsJSON:abc123")
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("PUT /app/rest/projects/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("DELETE /app/rest/projects/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Build Types (Jobs)
	ts.Handle("GET /app/rest/buildTypes", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "NonExistentJob123456") {
			Error(w, http.StatusNotFound, "No build types found by locator 'id:NonExistentJob123456'")
			return
		}
		JSON(w, api.BuildTypeList{
			Count: 1,
			BuildTypes: []api.BuildType{
				{ID: "TestProject_Build", Name: "Build", ProjectID: "TestProject"},
			},
		})
	})

	ts.Handle("GET /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		id := ExtractID(r.URL.Path, "id:")
		if id == "NonExistentJob123456" {
			Error(w, http.StatusNotFound, "No build types found by locator 'id:NonExistentJob123456'")
			return
		}

		if strings.Contains(r.URL.Path, "/snapshot-dependencies") {
			JSON(w, api.SnapshotDependencyList{Count: 0, SnapshotDependency: []api.SnapshotDependency{}})
			return
		}

		if strings.Contains(r.URL.Path, "/parameters/") {
			JSON(w, api.Parameter{Name: "param1", Value: "value1"})
			return
		}
		if strings.Contains(r.URL.Path, "/parameters") {
			JSON(w, api.ParameterList{
				Count:    1,
				Property: []api.Parameter{{Name: "param1", Value: "value1"}},
			})
			return
		}

		JSON(w, api.BuildType{
			ID:        id,
			Name:      "Build",
			ProjectID: "TestProject",
			WebURL:    ts.URL + "/viewType.html?buildTypeId=" + id,
		})
	})

	ts.Handle("PUT /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("POST /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("DELETE /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Builds
	ts.Handle("GET /app/rest/builds", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "id:999999999") {
			Error(w, http.StatusNotFound, "No build found by locator 'id:999999999'")
			return
		}
		JSON(w, api.BuildList{
			Count: 1,
			Builds: []api.Build{
				{
					ID:          1,
					Number:      "1",
					Status:      "SUCCESS",
					State:       "finished",
					BuildTypeID: "TestProject_Build",
					StartDate:   "20240101T120000+0000",
					FinishDate:  "20240101T120100+0000",
					WebURL:      ts.URL + "/viewLog.html?buildId=1",
				},
			},
		})
	})

	ts.Handle("GET /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		id := ExtractID(r.URL.Path, "id:")
		if id == "999999999" {
			Error(w, http.StatusNotFound, "No build found by locator 'id:999999999'")
			return
		}

		if strings.Contains(r.URL.Path, "/snapshot-dependencies") {
			JSON(w, api.BuildList{Count: 0, Builds: []api.Build{}})
			return
		}

		if strings.Contains(r.URL.Path, "/tags") {
			JSON(w, api.TagList{Tag: []api.Tag{{Name: "cli-test-tag"}, {Name: "another-tag"}}})
			return
		}
		if strings.Contains(r.URL.Path, "/comment") {
			Text(w, "CLI test comment")
			return
		}
		if strings.Contains(r.URL.Path, "/artifacts/content/") {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", "12")
			_, _ = w.Write([]byte("test content"))
			return
		}
		if strings.Contains(r.URL.Path, "/artifacts/children/logs") {
			JSON(w, api.Artifacts{
				Count: 2,
				File: []api.Artifact{
					{Name: "build.log", Size: 45678},
					{Name: "test.log", Size: 12345},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/artifacts/children/nonexistent") {
			Error(w, http.StatusNotFound, "Artifact not found: nonexistent")
			return
		}
		if strings.Contains(r.URL.Path, "/artifacts") {
			JSON(w, api.Artifacts{
				Count: 3,
				File: []api.Artifact{
					{Name: "build.jar", Size: 13002342},
					{Name: "test-report.html", Size: 239616},
					{Name: "logs", Children: &api.Artifacts{
						Count: 2,
						File: []api.Artifact{
							{Name: "build.log", Size: 45678},
							{Name: "test.log", Size: 12345},
						},
					}},
				},
			})
			return
		}

		JSON(w, api.Build{
			ID:          1,
			Number:      "1",
			Status:      "SUCCESS",
			State:       "running",
			BuildTypeID: "TestProject_Build",
			StartDate:   "20240101T120000+0000",
			WebURL:      ts.URL + "/viewLog.html?buildId=1",
		})
	})

	ts.Handle("POST /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("PUT /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("DELETE /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Build Queue
	ts.Handle("GET /app/rest/buildQueue", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.BuildQueue{Count: 0, Builds: []api.QueuedBuild{}})
	})

	ts.Handle("POST /app/rest/buildQueue", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.Build{
			ID:          100,
			Number:      "100",
			State:       "queued",
			BuildTypeID: "TestProject_Build",
			WebURL:      ts.URL + "/viewLog.html?buildId=100",
		})
	})

	ts.Handle("DELETE /app/rest/buildQueue/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	ts.Handle("PUT /app/rest/buildQueue/order/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("PUT /app/rest/buildQueue/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("GET /app/rest/buildQueue/id:", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/approval") {
			JSON(w, api.ApprovalInfo{Status: "waitingForApproval", CanBeApprovedByCurrentUser: true})
			return
		}
		JSON(w, api.QueuedBuild{ID: 100, State: "queued"})
	})

	// Build log
	ts.Handle("GET /downloadBuildLog.html", func(w http.ResponseWriter, r *http.Request) {
		Text(w, "[12:00:00] Build started\n[12:00:01] Compiling...\n[12:00:10] Build finished")
	})

	// Build messages (structured log API)
	ts.Handle("GET /app/messages", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.BuildMessagesResponse{
			Messages: []api.BuildMessage{
				{ID: 1, Text: "Build started", Level: 1, Status: 1, Timestamp: "2026-04-07T12:00:00.000+0000"},
				{ID: 2, Text: "Compiling...", Level: 2, Status: 1, Timestamp: "2026-04-07T12:00:01.000+0000"},
				{ID: 3, Text: "Build finished", Level: 1, Status: 1, Timestamp: "2026-04-07T12:00:10.000+0000"},
			},
			LastMessageIndex:    3,
			FocusIndex:          3,
			LastMessageIncluded: true,
		})
	})

	// Changes
	ts.Handle("GET /app/rest/changes", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.ChangeList{
			Count: 1,
			Change: []api.Change{
				{ID: 1, Version: "abc123", Username: "developer", Comment: "Fix bug"},
			},
		})
	})

	// Tests
	ts.Handle("GET /app/rest/testOccurrences", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.TestOccurrences{
			Count:  1,
			Passed: 1,
			TestOccurrence: []api.TestOccurrence{
				{ID: "1", Name: "TestExample", Status: "SUCCESS"},
			},
		})
	})

	// Problem occurrences
	ts.Handle("GET /app/rest/problemOccurrences", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.ProblemOccurrences{
			Count: 1,
			ProblemOccurrence: []api.ProblemOccurrence{
				{ID: "1", Type: "TC_COMPILATION_ERROR", Identity: "compilationError", Details: "Compilation failed with 3 errors"},
			},
		})
	})

	// Agents
	ts.Handle("GET /app/rest/agents", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.AgentList{
			Count: 2,
			Agents: []api.Agent{
				{ID: 1, Name: "Agent 1", Connected: true, Enabled: true, Authorized: true, Pool: &api.Pool{ID: 0, Name: "Default"}},
				{ID: 2, Name: "Agent 2", Connected: false, Enabled: true, Authorized: true, Pool: &api.Pool{ID: 0, Name: "Default"}},
			},
		})
	})

	ts.Handle("GET /app/rest/agents/id:", func(w http.ResponseWriter, r *http.Request) {
		id := ExtractID(r.URL.Path, "id:")
		if id == "999" {
			Error(w, http.StatusNotFound, "No agent found by locator 'id:999'")
			return
		}
		JSON(w, api.Agent{
			ID:         1,
			Name:       "Agent 1",
			Connected:  true,
			Enabled:    true,
			Authorized: true,
			WebURL:     ts.URL + "/agentDetails.html?id=1",
			Pool:       &api.Pool{ID: 0, Name: "Default"},
		})
	})

	ts.Handle("GET /app/rest/agents/name:", func(w http.ResponseWriter, r *http.Request) {
		name := ExtractID(r.URL.Path, "name:")
		if name == "NonExistentAgent" {
			Error(w, http.StatusNotFound, "No agent found by locator 'name:NonExistentAgent'")
			return
		}
		JSON(w, api.Agent{
			ID:         1,
			Name:       name,
			Connected:  true,
			Enabled:    true,
			Authorized: true,
			WebURL:     ts.URL + "/agentDetails.html?id=1",
			Pool:       &api.Pool{ID: 0, Name: "Default"},
		})
	})

	ts.Handle("PUT /app/rest/agents/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	ts.Handle("POST /remoteAccess/reboot.html", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			Error(w, http.StatusBadRequest, "Failed to parse form")
			return
		}
		agentID := r.FormValue("agent")
		if agentID == "999" {
			Error(w, http.StatusNotFound, "No agent found")
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("GET /app/rest/agents/id:1/compatibleBuildTypes", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.BuildTypeList{
			Count: 2,
			BuildTypes: []api.BuildType{
				{ID: "Project_Build", Name: "Build", ProjectName: "Project", ProjectID: "Project"},
				{ID: "Project_Test", Name: "Test", ProjectName: "Project", ProjectID: "Project"},
			},
		})
	})

	ts.Handle("GET /app/rest/agents/id:1/incompatibleBuildTypes", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.CompatibilityList{
			Count: 1,
			Compatibility: []api.Compatibility{
				{
					Compatible: false,
					BuildType:  &api.BuildType{ID: "OtherProject_Build", Name: "Build", ProjectName: "Other Project"},
					Reasons:    &api.IncompatibleReasons{Reasons: []string{"Missing requirement: docker"}},
				},
			},
		})
	})

	// Agent Pools
	ts.Handle("GET /app/rest/agentPools", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.PoolList{
			Count: 2,
			Pools: []api.Pool{
				{ID: 0, Name: "Default", MaxAgents: 0},
				{ID: 1, Name: "Linux Agents", MaxAgents: 10},
			},
		})
	})

	ts.Handle("GET /app/rest/agentPools/id:", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.Pool{
			ID:        0,
			Name:      "Default",
			MaxAgents: 0,
			Agents: &api.AgentList{
				Count: 1,
				Agents: []api.Agent{
					{ID: 1, Name: "Agent 1", Connected: true, Enabled: true, Authorized: true},
				},
			},
			Projects: &api.ProjectList{
				Count: 1,
				Projects: []api.Project{
					{ID: "_Root", Name: "Root project"},
				},
			},
		})
	})

	ts.Handle("POST /app/rest/agentPools/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("DELETE /app/rest/agentPools/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Versioned Settings
	ts.Handle("GET /app/rest/projects/TestProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.VersionedSettingsConfig{
			SynchronizationMode: "enabled",
			Format:              "kotlin",
			BuildSettingsMode:   "useFromVCS",
			VcsRootID:           "TestProject_HttpsGithubComExampleRepoGit",
			SettingsPath:        ".teamcity",
			AllowUIEditing:      true,
			ShowSettingsChanges: true,
		})
	})

	ts.Handle("GET /app/rest/projects/TestProject/versionedSettings/status", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.VersionedSettingsStatus{
			Type:        "info",
			Message:     "Settings are up to date",
			Timestamp:   "Mon Jan 27 10:30:00 UTC 2025",
			DslOutdated: false,
		})
	})

	ts.Handle("GET /app/rest/projects/WarningProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.VersionedSettingsConfig{
			SynchronizationMode: "enabled",
			Format:              "xml",
			BuildSettingsMode:   "useCurrentByDefault",
		})
	})

	ts.Handle("GET /app/rest/projects/WarningProject/versionedSettings/status", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.VersionedSettingsStatus{
			Type:        "warning",
			Message:     "DSL scripts need to be regenerated",
			Timestamp:   "Mon Jan 27 09:00:00 UTC 2025",
			DslOutdated: true,
		})
	})

	ts.Handle("GET /app/rest/projects/NoSettingsProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) {
		Error(w, http.StatusNotFound, "Versioned settings are not configured for this project")
	})

	ts.Handle("GET /app/rest/projects/NoSettingsProject/versionedSettings/status", func(w http.ResponseWriter, r *http.Request) {
		Error(w, http.StatusNotFound, "Versioned settings are not configured for this project")
	})

	// Cloud Profiles
	ts.Handle("GET /app/rest/cloud/profiles", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.CloudProfileList{
			Count: 2,
			Profiles: []api.CloudProfile{
				{ID: "aws-prod", Name: "AWS Production", CloudProviderID: "amazon", Project: &api.Project{ID: "TestProject", Name: "Test Project"}},
				{ID: "azure-eu", Name: "Azure EU", CloudProviderID: "azure", Project: &api.Project{ID: "TestProject", Name: "Test Project"}},
			},
		})
	})

	ts.Handle("GET /app/rest/cloud/profiles/", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.CloudProfile{
			ID: "aws-prod", Name: "AWS Production", CloudProviderID: "amazon", Project: &api.Project{ID: "TestProject", Name: "Test Project"},
		})
	})

	// Cloud Images
	ts.Handle("GET /app/rest/cloud/images", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.CloudImageList{
			Count: 2,
			Images: []api.CloudImage{
				{ID: "id:img-1,profileId:aws-prod", Name: "ubuntu-22-large", Profile: &api.CloudProfile{ID: "aws-prod", Name: "AWS Production"}},
				{ID: "id:img-2,profileId:azure-eu", Name: "windows-2022", Profile: &api.CloudProfile{ID: "azure-eu", Name: "Azure EU"}},
			},
		})
	})

	// Resulting properties (for run diff)
	ts.Handle("GET /app/rest/builds/id:1/resulting-properties", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.ParameterList{
			Count: 2,
			Property: []api.Parameter{
				{Name: "version", Value: "1.0.0"},
				{Name: "env.JAVA_HOME", Value: "/usr/lib/jvm/java-11"},
			},
		})
	})

	ts.Handle("GET /app/rest/builds/id:2/resulting-properties", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.ParameterList{
			Count: 3,
			Property: []api.Parameter{
				{Name: "version", Value: "1.0.1"},
				{Name: "env.JAVA_HOME", Value: "/usr/lib/jvm/java-17"},
				{Name: "new.feature", Value: "enabled"},
			},
		})
	})

	ts.Handle("GET /app/rest/cloud/images/", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.CloudImage{
			ID: "id:img-1,profileId:aws-prod", Name: "ubuntu-22-large",
			Profile: &api.CloudProfile{ID: "aws-prod", Name: "AWS Production"},
		})
	})

	// Cloud Instances
	ts.Handle("GET /app/rest/cloud/instances", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.CloudInstanceList{
			Count: 1,
			Instances: []api.CloudInstance{
				{
					ID: "i-0245b46070c443201", Name: "agent-cloud-1", State: "running",
					StartDate: "20240101T120000+0000",
					Image:     &api.CloudImage{ID: "id:img-1,profileId:aws-prod", Name: "ubuntu-22-large"},
					Agent:     &api.Agent{ID: 10, Name: "agent-cloud-1"},
				},
			},
		})
	})

	ts.Handle("GET /app/rest/cloud/instances/", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.CloudInstance{
			ID: "i-0245b46070c443201", Name: "agent-cloud-1", State: "running",
			StartDate: "20240101T120000+0000",
			Image:     &api.CloudImage{ID: "id:img-1,profileId:aws-prod", Name: "ubuntu-22-large"},
			Agent:     &api.Agent{ID: 10, Name: "agent-cloud-1"},
		})
	})

	ts.Handle("POST /app/rest/cloud/instances", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/actions/") {
			w.WriteHeader(http.StatusOK)
			return
		}
		JSON(w, api.CloudInstance{
			ID: "i-new-instance", Name: "agent-cloud-new", State: "starting",
			Image: &api.CloudImage{ID: "id:img-1,profileId:aws-prod", Name: "ubuntu-22-large"},
		})
	})

	// VCS Roots
	ts.Handle("GET /app/rest/vcs-roots", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.VcsRootList{
			Count: 1,
			VcsRoot: []api.VcsRoot{
				{
					ID:      "TestProject_Repo",
					Name:    "My Repo",
					VcsName: "jetbrains.git",
					Project: &api.Project{ID: "TestProject", Name: "Test Project"},
				},
			},
		})
	})

	ts.Handle("GET /app/rest/vcs-roots/id:", func(w http.ResponseWriter, r *http.Request) {
		id := ExtractID(r.URL.Path, "id:")
		if id == "NonExistentVcsRoot123456" {
			Error(w, http.StatusNotFound, "No VCS root found by locator 'id:NonExistentVcsRoot123456'")
			return
		}
		JSON(w, api.VcsRoot{
			ID:      id,
			Name:    "My Repo",
			VcsName: "jetbrains.git",
			Project: &api.Project{ID: "TestProject", Name: "Test Project"},
			Properties: &api.PropertyList{
				Property: []api.Property{
					{Name: "url", Value: "https://github.com/org/repo"},
					{Name: "branch", Value: "refs/heads/main"},
					{Name: "authMethod", Value: "PASSWORD"},
					{Name: "secure:password"},
				},
			},
		})
	})

	ts.Handle("POST /app/rest/vcs-roots", func(w http.ResponseWriter, r *http.Request) {
		var root api.VcsRoot
		if err := json.NewDecoder(r.Body).Decode(&root); err != nil {
			Error(w, http.StatusBadRequest, err.Error())
			return
		}
		root.ID = "TestProject_NewRoot"
		root.Href = "/app/rest/vcs-roots/id:TestProject_NewRoot"
		JSON(w, root)
	})

	ts.Handle("DELETE /app/rest/vcs-roots/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Test VCS Connection
	ts.Handle("POST /app/pipeline/repository/testConnection", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.TestConnectionResult{Status: "OK"})
	})

	// SSH Keys
	ts.Handle("GET /app/rest/projects/id:TestProject/sshKeys", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.SSHKeyList{
			SSHKey: []api.SSHKey{
				{Name: "deploy-key", Encrypted: false, PublicKey: "ssh-ed25519 AAAAC3..."},
				{Name: "backup-key", Encrypted: true, PublicKey: "ssh-rsa AAAAB3..."},
			},
		})
	})

	ts.Handle("GET /app/rest/projects/id:_Root/sshKeys", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.SSHKeyList{SSHKey: []api.SSHKey{}})
	})

	ts.Handle("POST /app/rest/projects/id:TestProject/sshKeys/generated", func(w http.ResponseWriter, r *http.Request) {
		keyName := r.URL.Query().Get("keyName")
		JSON(w, api.SSHKey{
			Name:      keyName,
			Encrypted: false,
			PublicKey: "ssh-ed25519 AAAAC3_generated_key",
			Project:   &api.Project{ID: "TestProject", Name: "Test Project"},
		})
	})

	ts.Handle("POST /app/rest/projects/id:TestProject/sshKeys/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("DELETE /app/rest/projects/id:TestProject/sshKeys/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Project Connections
	ts.Handle("GET /app/rest/projects/id:TestProject/projectFeatures", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.ProjectFeatureList{
			Count: 1,
			ProjectFeature: []api.ProjectFeature{
				{
					ID:   "PROJECT_EXT_1",
					Type: "OAuthProvider",
					Properties: &api.PropertyList{
						Property: []api.Property{
							{Name: "displayName", Value: "GitHub App"},
							{Name: "providerType", Value: "GitHubApp"},
						},
					},
				},
			},
		})
	})

	ts.Handle("GET /app/rest/projects/id:_Root/projectFeatures", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.ProjectFeatureList{Count: 0, ProjectFeature: []api.ProjectFeature{}})
	})

	ts.Handle("POST /app/rest/projects/id:TestProject/projectFeatures", func(w http.ResponseWriter, r *http.Request) {
		var feat api.ProjectFeature
		if err := json.NewDecoder(r.Body).Decode(&feat); err != nil {
			Error(w, http.StatusBadRequest, err.Error())
			return
		}
		feat.ID = "PROJECT_EXT_42"
		JSON(w, feat)
	})

	ts.Handle("POST /app/rest/projects/id:_Root/projectFeatures", func(w http.ResponseWriter, r *http.Request) {
		var feat api.ProjectFeature
		if err := json.NewDecoder(r.Body).Decode(&feat); err != nil {
			Error(w, http.StatusBadRequest, err.Error())
			return
		}
		feat.ID = "PROJECT_EXT_42"
		JSON(w, feat)
	})

	ts.Handle("DELETE /app/rest/projects/id:TestProject/projectFeatures/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	ts.Handle("DELETE /app/rest/projects/id:_Root/projectFeatures/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Pipelines
	ts.Handle("GET /app/rest/pipelines", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.PipelineList{
			Count: 1,
			Pipelines: []api.Pipeline{
				{
					ID:   "TestProject_CI",
					Name: "CI",
					ParentProject: &api.ProjectRef{
						ID:   "TestProject",
						Name: "Test Project",
					},
					HeadBuildType: &api.BuildTypeRef{ID: "TestProject_CI"},
					Jobs: &api.PipelineJobs{
						Count: 2,
						Job: []api.PipelineJob{
							{ID: "build", Name: "Build"},
							{ID: "test", Name: "Test"},
						},
					},
				},
			},
		})
	})

	ts.Handle("GET /app/rest/pipelines/", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.Pipeline{
			ID:   "TestProject_CI",
			Name: "CI",
			ParentProject: &api.ProjectRef{
				ID:   "TestProject",
				Name: "Test Project",
			},
			HeadBuildType: &api.BuildTypeRef{ID: "TestProject_CI"},
			Jobs: &api.PipelineJobs{
				Count: 2,
				Job: []api.PipelineJob{
					{ID: "build", Name: "Build"},
					{ID: "test", Name: "Test"},
				},
			},
		})
	})

	ts.Handle("POST /app/pipeline/schema/generate", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		})
	})

	config.SetUserForServer(ts.URL, "admin")

	return ts
}
