package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/config"
)

// TestServer wraps httptest.Server for easy API testing.
type TestServer struct {
	*httptest.Server
	handlers map[string]http.HandlerFunc
	t        *testing.T
}

// NewTestServer creates a test server and configures the client.
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	ts := &TestServer{
		handlers: make(map[string]http.HandlerFunc),
		t:        t,
	}

	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if h, ok := ts.handlers[key]; ok {
			h(w, r)
			return
		}

		var bestMatch string
		var bestHandler http.HandlerFunc
		for pattern, h := range ts.handlers {
			parts := strings.SplitN(pattern, " ", 2)
			if len(parts) != 2 {
				continue
			}
			method, path := parts[0], parts[1]
			if r.Method == method && strings.HasPrefix(r.URL.Path, path) {
				if len(path) > len(bestMatch) {
					bestMatch = path
					bestHandler = h
				}
			}
		}
		if bestHandler != nil {
			bestHandler(w, r)
			return
		}

		// Default: 404
		t.Logf("Unhandled request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))

	t.Setenv("TEAMCITY_URL", ts.URL)
	t.Setenv("TEAMCITY_TOKEN", "test-token")
	t.Setenv("TC_INSECURE_SKIP_WARN", "1")
	config.Init()

	original := cmd.GetClientFunc
	cmd.GetClientFunc = func() (api.ClientInterface, error) {
		return api.NewClient(ts.URL, "test-token"), nil
	}

	t.Cleanup(func() {
		ts.Close()
		cmd.GetClientFunc = original
	})

	return ts
}

// Handle registers a handler for "METHOD /path" pattern.
func (ts *TestServer) Handle(pattern string, h http.HandlerFunc) {
	ts.handlers[pattern] = h
}

// JSON writes a JSON response with 200 OK.
func JSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// JSONStatus writes a JSON response with specified status code.
func JSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Text writes a plain text response.
func Text(w http.ResponseWriter, s string) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(s))
}

// Error writes an API error response.
func Error(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.APIErrorResponse{
		Errors: []api.APIError{{Message: message}},
	})
}

// extractID extracts an ID from a path like /app/rest/builds/id:123/something
func extractID(path, prefix string) string {
	_, after, ok := strings.Cut(path, prefix)
	if !ok {
		return ""
	}
	rest := after
	// ID ends at / or ? or end of string
	end := strings.IndexAny(rest, "/?")
	if end == -1 {
		return rest
	}
	return rest[:end]
}

// setupMockClient creates a test server with all common API endpoints pre-registered.
func setupMockClient(t *testing.T) *TestServer {
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

	// Generic fallback for arbitrary API paths (for raw API command tests)
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
		// Check for a specific project lookup
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
		JSON(w, api.Project{ID: req.ID, Name: req.Name})
	})

	// Projects by ID - consolidated handler
	ts.Handle("GET /app/rest/projects/id:", func(w http.ResponseWriter, r *http.Request) {
		id := extractID(r.URL.Path, "id:")
		if id == "NonExistentProject123456" {
			Error(w, http.StatusNotFound, "No project found by locator 'id:NonExistentProject123456'")
			return
		}

		// Handle sub-paths
		if strings.Contains(r.URL.Path, "/parameters/") {
			// Get specific parameter
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

		// Default: return project
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

	// Handle projects without id: prefix (used by secure token API)
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

	// Build Types (Jobs) list
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

	// Build Types by ID - consolidated handler
	ts.Handle("GET /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		id := extractID(r.URL.Path, "id:")
		if id == "NonExistentJob123456" {
			Error(w, http.StatusNotFound, "No build types found by locator 'id:NonExistentJob123456'")
			return
		}

		// Handle parameters subpath
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

	// Builds list
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

	// Builds by ID - consolidated handler
	ts.Handle("GET /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		id := extractID(r.URL.Path, "id:")
		if id == "999999999" {
			Error(w, http.StatusNotFound, "No build found by locator 'id:999999999'")
			return
		}

		// Handle sub-paths
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
		// Cancel, tags, etc.
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
		id := extractID(r.URL.Path, "id:")
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
		name := extractID(r.URL.Path, "name:")
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

	// Agent reboot endpoint (form-based, not REST)
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

	// Set user for the test server URL (supports @me in run list)
	config.SetUserForServer(ts.URL, "admin")

	return ts
}
