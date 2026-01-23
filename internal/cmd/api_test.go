package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/config"
)

// createTestRootCmd creates a fresh root command with the api subcommand for testing
func createTestRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use: "tc",
	}
	rootCmd.PersistentFlags().Bool("no-color", false, "")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "")
	rootCmd.PersistentFlags().Bool("verbose", false, "")
	rootCmd.PersistentFlags().Bool("no-input", false, "")
	rootCmd.AddCommand(newAPICmd())
	return rootCmd
}

func setupMockServerForAPI(handler http.HandlerFunc) (*httptest.Server, func()) {
	server := httptest.NewServer(handler)

	// Save and override config
	originalURL := os.Getenv("TEAMCITY_URL")
	originalToken := os.Getenv("TEAMCITY_TOKEN")

	os.Setenv("TEAMCITY_URL", server.URL)
	os.Setenv("TEAMCITY_TOKEN", "test-token")
	config.Init()

	cleanup := func() {
		server.Close()
		os.Setenv("TEAMCITY_URL", originalURL)
		os.Setenv("TEAMCITY_TOKEN", originalToken)
		config.Init()
	}

	return server, cleanup
}

func TestAPICommandBasicGET(t *testing.T) {
	requestReceived := false
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/server" {
			t.Errorf("Expected /app/rest/server, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("Expected Bearer auth header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"version": "2024.03"})
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !requestReceived {
		t.Error("Expected request to be sent to server")
	}
}

func TestAPICommandPOSTWithFields(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["buildType"] != "MyBuild" {
			t.Errorf("Expected buildType MyBuild, got %v", body["buildType"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]int{"id": 123})
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/buildQueue", "-X", "POST", "-f", "buildType=MyBuild"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandWithCustomHeaders(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/xml" {
			t.Errorf("Expected Accept application/xml, got %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("X-Custom") != "custom-value" {
			t.Errorf("Expected X-Custom header, got %s", r.Header.Get("X-Custom"))
		}
		w.Write([]byte("<server/>"))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "-H", "Accept: application/xml", "-H", "X-Custom: custom-value"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandIncludeHeaders(t *testing.T) {
	requestReceived := false
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.Header().Set("X-Response-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "--include"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !requestReceived {
		t.Error("Expected request to be sent to server")
	}
	// Note: output includes headers is printed to stdout, not captured in buffer
}

func TestAPICommandSilentMode(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "--silent"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Silent mode should produce no output on success
	if out.String() != "" {
		t.Errorf("Expected no output in silent mode, got: %s", out.String())
	}
}

func TestAPICommandRawOutput(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"compact":true}`))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "--raw"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Raw mode should not pretty-print (no indentation)
	output := out.String()
	if strings.Contains(output, "  \"compact\"") {
		t.Errorf("Expected compact output in raw mode, got: %s", output)
	}
}

func TestAPICommandErrorResponse(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Resource not found"))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds/id:999"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected error for 404 response")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected error to mention 404, got: %v", err)
	}
}

func TestAPICommandDELETE(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds/id:123", "-X", "DELETE"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandPUT(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated":true}`))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/projects/id:Test", "-X", "PUT", "-f", "name=Updated"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandInvalidHeaderFormat(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "-H", "InvalidHeader"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected error for invalid header format")
	}
	if !strings.Contains(err.Error(), "invalid header format") {
		t.Errorf("Expected 'invalid header format' error, got: %v", err)
	}
}

func TestAPICommandInvalidFieldFormat(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "-X", "POST", "-f", "invalid"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected error for invalid field format")
	}
	if !strings.Contains(err.Error(), "invalid field format") {
		t.Errorf("Expected 'invalid field format' error, got: %v", err)
	}
}

func TestAPICommandWithJSONField(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		// Check that nested JSON was parsed correctly
		buildType, ok := body["buildType"].(map[string]interface{})
		if !ok {
			t.Errorf("Expected buildType to be object, got %T", body["buildType"])
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if buildType["id"] != "MyBuild" {
			t.Errorf("Expected buildType.id MyBuild, got %v", buildType["id"])
		}

		w.WriteHeader(http.StatusCreated)
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/buildQueue", "-X", "POST", "-f", `buildType={"id":"MyBuild"}`})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandFromStdin(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"test":"stdin"}` {
			t.Errorf("Unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusCreated)
	})
	defer cleanup()

	// Save original stdin
	oldStdin := os.Stdin

	// Create a pipe for stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.Write([]byte(`{"test":"stdin"}`))
	w.Close()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "-X", "POST", "--input", "-"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()

	// Restore stdin
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandPaginate(t *testing.T) {
	pageNum := 0
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		pageNum++
		w.Header().Set("Content-Type", "application/json")

		switch pageNum {
		case 1:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":    2,
				"nextHref": "/app/rest/builds?start=2",
				"build":    []map[string]int{{"id": 1}, {"id": 2}},
			})
		case 2:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count": 1,
				"build": []map[string]int{{"id": 3}},
			})
		}
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "--paginate"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if pageNum != 2 {
		t.Errorf("Expected 2 requests, got %d", pageNum)
	}
}

func TestAPICommandPaginateNoNextHref(t *testing.T) {
	requestCount := 0
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count": 2,
			"build": []map[string]int{{"id": 1}, {"id": 2}},
		})
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "--paginate"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("Expected 1 request (no pagination needed), got %d", requestCount)
	}
}

func TestAPICommandSlurp(t *testing.T) {
	pageNum := 0
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		pageNum++
		w.Header().Set("Content-Type", "application/json")

		switch pageNum {
		case 1:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":    2,
				"nextHref": "/app/rest/builds?start=2",
				"build":    []map[string]int{{"id": 1}, {"id": 2}},
			})
		case 2:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count": 1,
				"build": []map[string]int{{"id": 3}},
			})
		}
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "--paginate", "--slurp"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Note: output goes to stdout, not the buffer, but we verify the server was hit correctly
	if pageNum != 2 {
		t.Errorf("Expected 2 requests, got %d", pageNum)
	}
}

func TestAPICommandSlurpRequiresPaginate(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "--slurp"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected error when using --slurp without --paginate")
	}
}

func TestAPICommandPaginateOnlyGET(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "-X", "POST", "--paginate"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected error when using --paginate with POST")
	}
	if !strings.Contains(err.Error(), "only be used with GET") {
		t.Errorf("Expected error about GET only, got: %v", err)
	}
}

func TestAPICommandPaginateNonJSON(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte("<builds><build id='1'/></builds>"))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "--paginate"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected error when using --paginate with non-JSON response")
	}
	if !strings.Contains(err.Error(), "--paginate requires JSON response") {
		t.Errorf("Expected error about JSON requirement, got: %v", err)
	}
}

// Unit tests for pagination functions
func TestExtractNextHref(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name: "has nextHref",
			data: `{"count":100,"nextHref":"/app/rest/builds?start=100","build":[]}`,
			want: "/app/rest/builds?start=100",
		},
		{
			name: "no nextHref",
			data: `{"count":50,"build":[]}`,
			want: "",
		},
		{
			name: "empty nextHref",
			data: `{"count":50,"nextHref":"","build":[]}`,
			want: "",
		},
		{
			name:    "invalid json",
			data:    `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractNextHref([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("extractNextHref() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractNextHref() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectArrayKey(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name: "builds response",
			data: `{"count":2,"build":[{"id":1},{"id":2}]}`,
			want: "build",
		},
		{
			name: "buildTypes response",
			data: `{"count":2,"buildType":[{"id":"bt1"},{"id":"bt2"}]}`,
			want: "buildType",
		},
		{
			name: "projects response",
			data: `{"count":2,"project":[{"id":"p1"},{"id":"p2"}]}`,
			want: "project",
		},
		{
			name: "agents response",
			data: `{"count":1,"agent":[{"id":1}]}`,
			want: "agent",
		},
		{
			name: "no array key (single object)",
			data: `{"id":1,"name":"test"}`,
			want: "",
		},
		{
			name: "empty array",
			data: `{"count":0,"build":[]}`,
			want: "build",
		},
		{
			name:    "invalid json",
			data:    `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detectArrayKey([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("detectArrayKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("detectArrayKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractArrayItems(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		key     string
		wantLen int
		wantErr bool
	}{
		{
			name:    "extract builds",
			data:    `{"count":2,"build":[{"id":1},{"id":2}]}`,
			key:     "build",
			wantLen: 2,
		},
		{
			name:    "key not found",
			data:    `{"count":0,"build":[]}`,
			key:     "project",
			wantLen: 0,
		},
		{
			name:    "empty array",
			data:    `{"count":0,"build":[]}`,
			key:     "build",
			wantLen: 0,
		},
		{
			name:    "invalid json",
			data:    `not json`,
			key:     "build",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractArrayItems([]byte(tt.data), tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractArrayItems() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("extractArrayItems() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestMergePages(t *testing.T) {
	tests := []struct {
		name     string
		pages    []string
		arrayKey string
		want     string
		wantErr  bool
	}{
		{
			name: "merge two pages",
			pages: []string{
				`{"count":2,"build":[{"id":1},{"id":2}]}`,
				`{"count":2,"build":[{"id":3},{"id":4}]}`,
			},
			arrayKey: "build",
			want:     `[{"id":1},{"id":2},{"id":3},{"id":4}]`,
		},
		{
			name: "single page",
			pages: []string{
				`{"count":2,"build":[{"id":1},{"id":2}]}`,
			},
			arrayKey: "build",
			want:     `[{"id":1},{"id":2}]`,
		},
		{
			name: "empty pages",
			pages: []string{
				`{"count":0,"build":[]}`,
				`{"count":0,"build":[]}`,
			},
			arrayKey: "build",
			want:     `[]`,
		},
		{
			name: "mixed sizes",
			pages: []string{
				`{"count":3,"build":[{"id":1},{"id":2},{"id":3}]}`,
				`{"count":1,"build":[{"id":4}]}`,
			},
			arrayKey: "build",
			want:     `[{"id":1},{"id":2},{"id":3},{"id":4}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pages [][]byte
			for _, p := range tt.pages {
				pages = append(pages, []byte(p))
			}

			got, err := mergePages(pages, tt.arrayKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("mergePages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Compare as JSON to ignore whitespace differences
			var gotJSON, wantJSON interface{}
			json.Unmarshal(got, &gotJSON)
			json.Unmarshal([]byte(tt.want), &wantJSON)

			gotBytes, _ := json.Marshal(gotJSON)
			wantBytes, _ := json.Marshal(wantJSON)

			if string(gotBytes) != string(wantBytes) {
				t.Errorf("mergePages() = %s, want %s", string(got), tt.want)
			}
		})
	}
}

func TestFetchAllPages(t *testing.T) {
	pageNum := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageNum++
		w.Header().Set("Content-Type", "application/json")

		switch pageNum {
		case 1:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":    2,
				"nextHref": "/app/rest/builds?start=2",
				"build":    []map[string]int{{"id": 1}, {"id": 2}},
			})
		case 2:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count":    2,
				"nextHref": "/app/rest/builds?start=4",
				"build":    []map[string]int{{"id": 3}, {"id": 4}},
			})
		case 3:
			// Last page, no nextHref
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count": 1,
				"build": []map[string]int{{"id": 5}},
			})
		}
	}))
	defer server.Close()

	// Set up config
	originalURL := os.Getenv("TEAMCITY_URL")
	originalToken := os.Getenv("TEAMCITY_TOKEN")
	os.Setenv("TEAMCITY_URL", server.URL)
	os.Setenv("TEAMCITY_TOKEN", "test-token")
	config.Init()
	defer func() {
		os.Setenv("TEAMCITY_URL", originalURL)
		os.Setenv("TEAMCITY_TOKEN", originalToken)
		config.Init()
	}()

	client, err := getClient()
	if err != nil {
		t.Fatalf("Failed to get client: %v", err)
	}

	pages, err := fetchAllPages(client, "/app/rest/builds", nil)
	if err != nil {
		t.Fatalf("fetchAllPages() error = %v", err)
	}

	if len(pages) != 3 {
		t.Errorf("fetchAllPages() returned %d pages, want 3", len(pages))
	}

	// Verify we can extract items from all pages
	arrayKey, _ := detectArrayKey(pages[0])
	merged, err := mergePages(pages, arrayKey)
	if err != nil {
		t.Fatalf("mergePages() error = %v", err)
	}

	var items []map[string]int
	json.Unmarshal(merged, &items)
	if len(items) != 5 {
		t.Errorf("merged result has %d items, want 5", len(items))
	}
}

func TestFetchAllPagesSinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Single page, no nextHref
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count": 2,
			"build": []map[string]int{{"id": 1}, {"id": 2}},
		})
	}))
	defer server.Close()

	// Set up config
	originalURL := os.Getenv("TEAMCITY_URL")
	originalToken := os.Getenv("TEAMCITY_TOKEN")
	os.Setenv("TEAMCITY_URL", server.URL)
	os.Setenv("TEAMCITY_TOKEN", "test-token")
	config.Init()
	defer func() {
		os.Setenv("TEAMCITY_URL", originalURL)
		os.Setenv("TEAMCITY_TOKEN", originalToken)
		config.Init()
	}()

	client, err := getClient()
	if err != nil {
		t.Fatalf("Failed to get client: %v", err)
	}

	pages, err := fetchAllPages(client, "/app/rest/builds", nil)
	if err != nil {
		t.Fatalf("fetchAllPages() error = %v", err)
	}

	if len(pages) != 1 {
		t.Errorf("fetchAllPages() returned %d pages, want 1", len(pages))
	}
}
