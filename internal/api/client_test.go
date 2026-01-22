package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://example.com", "test-token")

	if client.BaseURL != "https://example.com" {
		t.Errorf("Expected BaseURL https://example.com, got %s", client.BaseURL)
	}
	if client.Token != "test-token" {
		t.Errorf("Expected Token test-token, got %s", client.Token)
	}
}

func TestNewClientTrimsTrailingSlash(t *testing.T) {
	client := NewClient("https://example.com/", "test-token")

	if client.BaseURL != "https://example.com" {
		t.Errorf("Expected BaseURL without trailing slash, got %s", client.BaseURL)
	}
}

func TestApiPath(t *testing.T) {
	client := NewClient("https://example.com", "token")
	result := client.apiPath("/app/rest/builds")
	if result != "/app/rest/builds" {
		t.Errorf("Expected /app/rest/builds, got %s", result)
	}

	// Test with APIVersion set manually
	client.APIVersion = "2023.1"
	result = client.apiPath("/app/rest/builds")
	if result != "/app/rest/2023.1/builds" {
		t.Errorf("Expected /app/rest/2023.1/builds, got %s", result)
	}

	// Non-rest path should not be modified
	result = client.apiPath("/downloadBuildLog.html")
	if result != "/downloadBuildLog.html" {
		t.Errorf("Expected /downloadBuildLog.html, got %s", result)
	}
}

func TestClientOptions(t *testing.T) {
	client := NewClient("https://example.com", "token", WithAPIVersion("2023.1"), WithTimeout(60*time.Second))
	if client.APIVersion != "2023.1" {
		t.Errorf("Expected APIVersion 2023.1, got %s", client.APIVersion)
	}
	if client.HTTPClient.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", client.HTTPClient.Timeout)
	}
}

func TestGetParameterValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("param-value"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	val, err := client.GetParameterValue("/app/rest/projects/id:Test/parameters/name/value")
	if err != nil {
		t.Fatalf("GetParameterValue failed: %v", err)
	}
	if val != "param-value" {
		t.Errorf("Expected param-value, got %s", val)
	}
}

func TestPinBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/builds/id:123/pin" {
			t.Errorf("Expected /app/rest/builds/id:123/pin, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Bearer auth header")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.PinBuild("123", "Test comment")
	if err != nil {
		t.Fatalf("PinBuild failed: %v", err)
	}
}

func TestUnpinBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/builds/id:456/pin" {
			t.Errorf("Expected /app/rest/builds/id:456/pin, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.UnpinBuild("456")
	if err != nil {
		t.Fatalf("UnpinBuild failed: %v", err)
	}
}

func TestAddBuildTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/builds/id:789/tags" {
			t.Errorf("Expected /app/rest/builds/id:789/tags, got %s", r.URL.Path)
		}

		var tags TagList
		if err := json.NewDecoder(r.Body).Decode(&tags); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}
		if len(tags.Tag) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(tags.Tag))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.AddBuildTags("789", []string{"tag1", "tag2"})
	if err != nil {
		t.Fatalf("AddBuildTags failed: %v", err)
	}
}

func TestGetBuildTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TagList{Tag: []Tag{{Name: "tag1"}, {Name: "tag2"}}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	tags, err := client.GetBuildTags("123")
	if err != nil {
		t.Fatalf("GetBuildTags failed: %v", err)
	}
	if len(tags.Tag) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(tags.Tag))
	}
}

func TestRemoveBuildTag(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: GET current tags
			if r.Method != "GET" {
				t.Errorf("First call: Expected GET, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"count":2,"tag":[{"name":"mytag"},{"name":"othertag"}]}`))
		} else {
			// Second call: PUT remaining tags
			if r.Method != "PUT" {
				t.Errorf("Second call: Expected PUT, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.RemoveBuildTag("123", "mytag")
	if err != nil {
		t.Fatalf("RemoveBuildTag failed: %v", err)
	}
}

func TestSetBuildComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("Expected Content-Type text/plain, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.SetBuildComment("123", "Test comment")
	if err != nil {
		t.Fatalf("SetBuildComment failed: %v", err)
	}
}

func TestGetBuildComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"comment":{"text":"Test comment"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	comment, err := client.GetBuildComment("123")
	if err != nil {
		t.Fatalf("GetBuildComment failed: %v", err)
	}
	if comment != "Test comment" {
		t.Errorf("Expected 'Test comment', got %q", comment)
	}
}

func TestGetBuildCommentNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Build exists but has no comment - returns empty comment object
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	comment, err := client.GetBuildComment("123")
	if err != nil {
		t.Fatalf("GetBuildComment failed: %v", err)
	}
	if comment != "" {
		t.Errorf("Expected empty comment, got %q", comment)
	}
}

func TestDeleteBuildComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.DeleteBuildComment("123")
	if err != nil {
		t.Fatalf("DeleteBuildComment failed: %v", err)
	}
}

func TestSetQueuedBuildPosition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/buildQueue/order/123" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("Expected Content-Type text/plain, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.SetQueuedBuildPosition("123", 5)
	if err != nil {
		t.Fatalf("SetQueuedBuildPosition failed: %v", err)
	}
}

func TestMoveQueuedBuildToTop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/rest/buildQueue/order/123" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.MoveQueuedBuildToTop("123")
	if err != nil {
		t.Fatalf("MoveQueuedBuildToTop failed: %v", err)
	}
}

func TestApproveQueuedBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/buildQueue/id:123/approval/status" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.ApproveQueuedBuild("123")
	if err != nil {
		t.Fatalf("ApproveQueuedBuild failed: %v", err)
	}
}

func TestGetQueuedBuildApprovalInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApprovalInfo{
			Status:             "waitingForApproval",
			ConfigurationValid: true,
			CanBeApproved:      true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	info, err := client.GetQueuedBuildApprovalInfo("123")
	if err != nil {
		t.Fatalf("GetQueuedBuildApprovalInfo failed: %v", err)
	}
	if info.Status != "waitingForApproval" {
		t.Errorf("Expected status waitingForApproval, got %s", info.Status)
	}
	if !info.CanBeApproved {
		t.Error("Expected CanBeApproved to be true")
	}
}

func TestCheckVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Server{
			Version:      "2024.03",
			VersionMajor: 2024,
			VersionMinor: 3,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.CheckVersion()
	if err != nil {
		t.Fatalf("CheckVersion failed: %v", err)
	}
}

func TestCheckVersionOldServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Server{
			Version:      "2019.1",
			VersionMajor: 2019,
			VersionMinor: 1,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.CheckVersion()
	if err == nil {
		t.Fatal("Expected CheckVersion to fail for old server")
	}
}

func TestHandleErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expectErr  string
	}{
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			expectErr:  "authentication failed",
		},
		{
			name:       "forbidden",
			statusCode: http.StatusForbidden,
			expectErr:  "permission denied",
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			expectErr:  "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte("error message"))
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			_, err := client.GetBuild("123")
			if err == nil {
				t.Fatal("Expected error")
			}
		})
	}
}

func TestCreateSecureToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/projects/Sandbox/secure/tokens" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("Expected Content-Type text/plain, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Accept") != "text/plain" {
			t.Errorf("Expected Accept text/plain, got %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("scrambled-token-123"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	token, err := client.CreateSecureToken("Sandbox", "my-secret-value")
	if err != nil {
		t.Fatalf("CreateSecureToken failed: %v", err)
	}
	if token != "scrambled-token-123" {
		t.Errorf("Expected token scrambled-token-123, got %s", token)
	}
}

func TestCreateSecureTokenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("permission denied"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.CreateSecureToken("Sandbox", "my-secret-value")
	if err == nil {
		t.Fatal("Expected error for forbidden response")
	}
}

func TestGetSecureValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/projects/Sandbox/secure/values/token-123" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "text/plain" {
			t.Errorf("Expected Accept text/plain, got %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("original-secret-value"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	value, err := client.GetSecureValue("Sandbox", "token-123")
	if err != nil {
		t.Fatalf("GetSecureValue failed: %v", err)
	}
	if value != "original-secret-value" {
		t.Errorf("Expected value original-secret-value, got %s", value)
	}
}

func TestGetSecureValueError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("token not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.GetSecureValue("Sandbox", "invalid-token")
	if err == nil {
		t.Fatal("Expected error for not found response")
	}
}

func TestGetProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ProjectList{
			Count: 2,
			Projects: []Project{
				{ID: "Project1", Name: "Project One"},
				{ID: "Project2", Name: "Project Two"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	projects, err := client.GetProjects(ProjectsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}
	if projects.Count != 2 {
		t.Errorf("Expected 2 projects, got %d", projects.Count)
	}
}

func TestGetProjectsWithParent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("locator")
		if !strings.Contains(query, "parentProject:_Root") {
			t.Errorf("Expected locator to contain parentProject:_Root, got %s", query)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ProjectList{Count: 1})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.GetProjects(ProjectsOptions{Parent: "_Root", Limit: 5})
	if err != nil {
		t.Fatalf("GetProjects with parent failed: %v", err)
	}
}

func TestGetProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/rest/projects/id:Sandbox" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Project{
			ID:   "Sandbox",
			Name: "Sandbox Project",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	project, err := client.GetProject("Sandbox")
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if project.ID != "Sandbox" {
		t.Errorf("Expected project ID Sandbox, got %s", project.ID)
	}
}

func TestGetBuildTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildTypeList{
			Count: 1,
			BuildTypes: []BuildType{
				{ID: "Config1", Name: "Config One"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	configs, err := client.GetBuildTypes(BuildTypesOptions{Limit: 10})
	if err != nil {
		t.Fatalf("GetBuildTypes failed: %v", err)
	}
	if configs.Count != 1 {
		t.Errorf("Expected 1 config, got %d", configs.Count)
	}
}

func TestGetBuildType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildType{
			ID:   "Sandbox_Demo",
			Name: "Demo Build",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	config, err := client.GetBuildType("Sandbox_Demo")
	if err != nil {
		t.Fatalf("GetBuildType failed: %v", err)
	}
	if config.ID != "Sandbox_Demo" {
		t.Errorf("Expected config ID Sandbox_Demo, got %s", config.ID)
	}
}

func TestPauseBuildType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/buildTypes/id:Sandbox_Demo/paused" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.PauseBuildType("Sandbox_Demo")
	if err != nil {
		t.Fatalf("PauseBuildType failed: %v", err)
	}
}

func TestResumeBuildType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.ResumeBuildType("Sandbox_Demo")
	if err != nil {
		t.Fatalf("ResumeBuildType failed: %v", err)
	}
}

func TestRemoveBuildTagNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return tags that don't contain the requested tag
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":1,"tag":[{"name":"othertag"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.RemoveBuildTag("123", "nonexistent")
	// The function returns an error when tag is not found - this is correct behavior
	if err == nil {
		t.Fatal("RemoveBuildTag should error for missing tag")
	}
}

func TestGetBuilds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildList{
			Count: 1,
			Builds: []Build{
				{ID: 123, Number: "42", Status: "SUCCESS"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	builds, err := client.GetBuilds(BuildsOptions{BuildTypeID: "Test", Limit: 10})
	if err != nil {
		t.Fatalf("GetBuilds failed: %v", err)
	}
	if builds.Count != 1 {
		t.Errorf("Expected 1 build, got %d", builds.Count)
	}
}

func TestGetBuildsWithAllFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL-decode the query to check filters
		locator, _ := url.QueryUnescape(r.URL.Query().Get("locator"))
		// Check that filters are applied (status is uppercase: SUCCESS)
		if !strings.Contains(locator, "status:SUCCESS") {
			t.Errorf("Expected status filter in locator: %s", locator)
		}
		if !strings.Contains(locator, "state:finished") {
			t.Errorf("Expected state filter in locator: %s", locator)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildList{Count: 0})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.GetBuilds(BuildsOptions{
		BuildTypeID: "Test",
		Status:      "SUCCESS",
		State:       "finished",
		Branch:      "main",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("GetBuilds with filters failed: %v", err)
	}
}

func TestGetBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Build{
			ID:     123,
			Number: "42",
			Status: "SUCCESS",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	build, err := client.GetBuild("123")
	if err != nil {
		t.Fatalf("GetBuild failed: %v", err)
	}
	if build.ID != 123 {
		t.Errorf("Expected build ID 123, got %d", build.ID)
	}
}

func TestGetBuildQueue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildList{
			Count: 0,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	queue, err := client.GetBuildQueue(QueueOptions{Limit: 10})
	if err != nil {
		t.Fatalf("GetBuildQueue failed: %v", err)
	}
	if queue.Count != 0 {
		t.Errorf("Expected 0 builds in queue, got %d", queue.Count)
	}
}

func TestRemoveFromQueue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.RemoveFromQueue("123")
	if err != nil {
		t.Fatalf("RemoveFromQueue failed: %v", err)
	}
}

func TestCancelBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Build{ID: 123})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.CancelBuild("123", "Test cancel")
	if err != nil {
		t.Fatalf("CancelBuild failed: %v", err)
	}
}

func TestGetCurrentUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(User{
			Username: "admin",
			Name:     "Administrator",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	user, err := client.GetCurrentUser()
	if err != nil {
		t.Fatalf("GetCurrentUser failed: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("Expected username admin, got %s", user.Username)
	}
}

func TestGetServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Server{
			Version:      "2024.03",
			VersionMajor: 2024,
			VersionMinor: 3,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	srv, err := client.GetServer()
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if srv.VersionMajor != 2024 {
		t.Errorf("Expected version major 2024, got %d", srv.VersionMajor)
	}
}

func TestGetProjectParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ParameterList{
			Count: 1,
			Property: []Parameter{
				{Name: "MY_PARAM", Value: "my_value"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	params, err := client.GetProjectParameters("Sandbox")
	if err != nil {
		t.Fatalf("GetProjectParameters failed: %v", err)
	}
	if params.Count != 1 {
		t.Errorf("Expected 1 parameter, got %d", params.Count)
	}
}

func TestSetProjectParameter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.SetProjectParameter("Sandbox", "MY_PARAM", "my_value", false)
	if err != nil {
		t.Fatalf("SetProjectParameter failed: %v", err)
	}
}

func TestSetProjectParameterSecure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var param Parameter
		json.NewDecoder(r.Body).Decode(&param)
		if param.Type == nil || param.Type.RawValue != "password" {
			t.Error("Expected secure parameter to have password type")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.SetProjectParameter("Sandbox", "SECRET", "secret_value", true)
	if err != nil {
		t.Fatalf("SetProjectParameter (secure) failed: %v", err)
	}
}

func TestDeleteProjectParameter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.DeleteProjectParameter("Sandbox", "MY_PARAM")
	if err != nil {
		t.Fatalf("DeleteProjectParameter failed: %v", err)
	}
}

func TestGetBuildTypeParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ParameterList{Count: 0})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	params, err := client.GetBuildTypeParameters("Sandbox_Demo")
	if err != nil {
		t.Fatalf("GetBuildTypeParameters failed: %v", err)
	}
	if params == nil {
		t.Error("Expected non-nil params")
	}
}

func TestGetBuildLog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Build log content here"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	log, err := client.GetBuildLog("123")
	if err != nil {
		t.Fatalf("GetBuildLog failed: %v", err)
	}
	if log != "Build log content here" {
		t.Errorf("Unexpected log content: %s", log)
	}
}

func TestGetArtifacts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Artifacts{
			Count: 1,
			File: []Artifact{
				{Name: "artifact.zip", Size: 1024},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	artifacts, err := client.GetArtifacts("123")
	if err != nil {
		t.Fatalf("GetArtifacts failed: %v", err)
	}
	if artifacts.Count != 1 {
		t.Errorf("Expected 1 artifact, got %d", artifacts.Count)
	}
}

func TestDownloadArtifact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("artifact content"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	data, err := client.DownloadArtifact("123", "test.txt")
	if err != nil {
		t.Fatalf("DownloadArtifact failed: %v", err)
	}
	if string(data) != "artifact content" {
		t.Errorf("Unexpected artifact content: %s", string(data))
	}
}

func TestParseTeamCityTime(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
	}{
		{
			input:    "20250710T080607+0000",
			expected: time.Date(2025, 7, 10, 8, 6, 7, 0, time.UTC),
		},
		{
			input:    "20240115T143022+0000",
			expected: time.Date(2024, 1, 15, 14, 30, 22, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, err := ParseTeamCityTime(tc.input)
			if err != nil {
				t.Fatalf("ParseTeamCityTime failed: %v", err)
			}
			if !result.Equal(tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestParseTeamCityTimeEmpty(t *testing.T) {
	result, err := ParseTeamCityTime("")
	// ParseTeamCityTime returns an error for invalid/empty strings
	if err == nil {
		t.Fatal("ParseTeamCityTime with empty string should error")
	}
	if !result.IsZero() {
		t.Errorf("Expected zero time for empty string, got %v", result)
	}
}

func TestRawRequestGET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/server" {
			t.Errorf("Expected /app/rest/server, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("Expected Bearer auth header")
		}
		w.Header().Set("X-Custom-Header", "custom-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"version":"2024.03"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	resp, err := client.RawRequest("GET", "/app/rest/server", nil, nil)
	if err != nil {
		t.Fatalf("RawRequest failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if resp.Headers.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("Expected custom header, got %s", resp.Headers.Get("X-Custom-Header"))
	}
	if string(resp.Body) != `{"version":"2024.03"}` {
		t.Errorf("Unexpected body: %s", string(resp.Body))
	}
}

func TestRawRequestPOST(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"test":"data"}` {
			t.Errorf("Unexpected body: %s", string(body))
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":123}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	body := strings.NewReader(`{"test":"data"}`)
	resp, err := client.RawRequest("POST", "/app/rest/builds", body, nil)
	if err != nil {
		t.Fatalf("RawRequest failed: %v", err)
	}

	if resp.StatusCode != 201 {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
}

func TestRawRequestWithCustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/xml" {
			t.Errorf("Expected Accept application/xml, got %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("X-Custom") != "value" {
			t.Errorf("Expected X-Custom header, got %s", r.Header.Get("X-Custom"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<server/>"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	headers := map[string]string{
		"Accept":   "application/xml",
		"X-Custom": "value",
	}
	resp, err := client.RawRequest("GET", "/app/rest/server", nil, headers)
	if err != nil {
		t.Fatalf("RawRequest failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "<server/>" {
		t.Errorf("Unexpected body: %s", string(resp.Body))
	}
}

func TestRawRequestErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Resource not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	resp, err := client.RawRequest("GET", "/app/rest/builds/id:999", nil, nil)
	if err != nil {
		t.Fatalf("RawRequest should not error on HTTP errors: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "Resource not found" {
		t.Errorf("Unexpected body: %s", string(resp.Body))
	}
}

func TestRawRequestDELETE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	resp, err := client.RawRequest("DELETE", "/app/rest/builds/id:123", nil, nil)
	if err != nil {
		t.Fatalf("RawRequest failed: %v", err)
	}

	if resp.StatusCode != 204 {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
}

func TestRawRequestPUT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	body := strings.NewReader(`{"name":"updated"}`)
	resp, err := client.RawRequest("PUT", "/app/rest/projects/id:Test", body, nil)
	if err != nil {
		t.Fatalf("RawRequest failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestExtractErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "valid error response",
			body:     `{"errors":[{"message":"No build types found by locator 'Test'."}]}`,
			expected: "job 'Test' not found",
		},
		{
			name:     "empty errors array",
			body:     `{"errors":[]}`,
			expected: "",
		},
		{
			name:     "malformed JSON",
			body:     `not json`,
			expected: "",
		},
		{
			name:     "empty body",
			body:     ``,
			expected: "",
		},
		{
			name:     "missing errors field",
			body:     `{"other":"field"}`,
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractErrorMessage([]byte(tc.body))
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestHumanizeErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "build type not found with period",
			input:    "No build types found by locator 'Sandbox_Demo'.",
			expected: "job 'Sandbox_Demo' not found",
		},
		{
			name:     "build type not found without period",
			input:    "No build types found by locator 'Sandbox_Demo'",
			expected: "job 'Sandbox_Demo' not found",
		},
		{
			name:     "build not found",
			input:    "No build found by locator '12345'.",
			expected: "run '12345' not found",
		},
		{
			name:     "project not found",
			input:    "No project found by locator 'MyProject'.",
			expected: "project 'MyProject' not found",
		},
		{
			name:     "nothing found with buildType locator",
			input:    "Nothing is found by locator 'count:1,buildType:(id:Sandbox_Demo)'.",
			expected: "no runs found for job 'Sandbox_Demo'",
		},
		{
			name:     "unrecognized message passes through",
			input:    "Some other error message",
			expected: "Some other error message",
		},
		{
			name:     "empty message",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := humanizeErrorMessage(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}
