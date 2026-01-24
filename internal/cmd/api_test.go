package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: API command tests cannot use t.Parallel() because they modify
// environment variables and shared config state.

// createTestRootCmd creates a fresh root command with the api subcommand for testing.
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

// setupMockServerForAPI creates a test server and configures the environment.
func setupMockServerForAPI(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)

	originalURL := os.Getenv("TEAMCITY_URL")
	originalToken := os.Getenv("TEAMCITY_TOKEN")

	os.Setenv("TEAMCITY_URL", server.URL)
	os.Setenv("TEAMCITY_TOKEN", "test-token")
	config.Init()

	t.Cleanup(func() {
		server.Close()
		os.Setenv("TEAMCITY_URL", originalURL)
		os.Setenv("TEAMCITY_TOKEN", originalToken)
		config.Init()
	})

	return server
}

func TestAPICommandBasicGET(T *testing.T) {
	requestReceived := false
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		assert.Equal(T, "GET", r.Method, "Method")
		assert.Equal(T, "/app/rest/server", r.URL.Path, "URL.Path")
		assert.Equal(T, "Bearer test-token", r.Header.Get("Authorization"), "Authorization header")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"version":      " (build 197398)",
			"versionMajor": 2025,
			"versionMinor": 7,
			"buildNumber":  "197398",
			"webUrl":       "http://mock.teamcity.test",
		})
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
	assert.True(T, requestReceived, "expected request to be sent to server")
}

func TestAPICommandPOSTWithFields(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(T, "POST", r.Method, "Method")
		assert.Equal(T, "application/json", r.Header.Get("Content-Type"), "Content-Type")

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(T, "MyBuild", body["buildType"], "body[buildType]")

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]int{"id": 123})
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/buildQueue", "-X", "POST", "-f", "buildType=MyBuild"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
}

func TestAPICommandWithCustomHeaders(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(T, "application/xml", r.Header.Get("Accept"), "Accept header")
		assert.Equal(T, "custom-value", r.Header.Get("X-Custom"), "X-Custom header")
		w.Write([]byte("<server/>"))
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "-H", "Accept: application/xml", "-H", "X-Custom: custom-value"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
}

func TestAPICommandIncludeHeaders(T *testing.T) {
	requestReceived := false
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.Header().Set("X-Response-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "--include"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
	assert.True(T, requestReceived, "expected request to be sent to server")
	// Note: output includes headers is printed to stdout, not captured in buffer
}

func TestAPICommandSilentMode(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "--silent"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)

	// Silent mode should produce no output on success
	assert.Empty(T, out.String(), "output in silent mode")
}

func TestAPICommandRawOutput(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"compact":true}`))
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "--raw"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)

	// Raw mode should not pretty-print (no indentation)
	assert.NotContains(T, out.String(), "  \"compact\"", "output in raw mode should be compact")
}

func TestAPICommandErrorResponse(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Resource not found"))
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds/id:999"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.Error(T, err, "expected error for 404 response")
	assert.Contains(T, err.Error(), "404")
}

func TestAPICommandDELETE(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(T, "DELETE", r.Method, "Method")
		w.WriteHeader(http.StatusNoContent)
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds/id:123", "-X", "DELETE"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
}

func TestAPICommandPUT(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(T, "PUT", r.Method, "Method")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated":true}`))
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/projects/id:Test", "-X", "PUT", "-f", "name=Updated"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
}

func TestAPICommandInvalidHeaderFormat(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "-H", "InvalidHeader"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.Error(T, err, "expected error for invalid header format")
	assert.Contains(T, err.Error(), "invalid header format")
}

func TestAPICommandInvalidFieldFormat(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "-X", "POST", "-f", "invalid"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.Error(T, err, "expected error for invalid field format")
	assert.Contains(T, err.Error(), "invalid field format")
}

func TestAPICommandWithJSONField(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		// Check that nested JSON was parsed correctly
		buildType, ok := body["buildType"].(map[string]interface{})
		require.True(T, ok, "body[buildType] should be a map")
		assert.Equal(T, "MyBuild", buildType["id"], "buildType[id]")

		w.WriteHeader(http.StatusCreated)
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/buildQueue", "-X", "POST", "-f", `buildType={"id":"MyBuild"}`})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
}

func TestAPICommandFromStdin(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Equal(T, `{"test":"stdin"}`, string(body), "request body")
		w.WriteHeader(http.StatusCreated)
	})

	// Save original stdin
	oldStdin := os.Stdin
	T.Cleanup(func() { os.Stdin = oldStdin })

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
	require.NoError(T, err)
}

func TestAPICommandPaginate(T *testing.T) {
	pageNum := 0
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
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

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "--paginate"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
	assert.Equal(T, 2, pageNum, "request count")
}

func TestAPICommandPaginateNoNextHref(T *testing.T) {
	requestCount := 0
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count": 2,
			"build": []map[string]int{{"id": 1}, {"id": 2}},
		})
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "--paginate"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)
	assert.Equal(T, 1, requestCount, "request count (no pagination needed)")
}

func TestAPICommandSlurp(T *testing.T) {
	pageNum := 0
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
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

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "--paginate", "--slurp"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.NoError(T, err)

	// Note: output goes to stdout, not the buffer, but we verify the server was hit correctly
	assert.Equal(T, 2, pageNum, "request count")
}

func TestAPICommandSlurpRequiresPaginate(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "--slurp"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.Error(T, err, "expected error when using --slurp without --paginate")
}

func TestAPICommandPaginateOnlyGET(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "-X", "POST", "--paginate"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.Error(T, err, "expected error when using --paginate with POST")
	assert.Contains(T, err.Error(), "only be used with GET")
}

func TestAPICommandPaginateNonJSON(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte("<builds><build id='1'/></builds>"))
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "--paginate"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.Error(T, err, "expected error for non-JSON response with --paginate")
	assert.Contains(T, err.Error(), "--paginate requires JSON response")
}

// Unit tests for pagination functions - these can run in parallel
func TestExtractNextHref(T *testing.T) {
	T.Parallel()

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

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := extractNextHref([]byte(tc.data))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDetectArrayKey(T *testing.T) {
	T.Parallel()

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

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := detectArrayKey([]byte(tc.data))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractArrayItems(T *testing.T) {
	T.Parallel()

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

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := extractArrayItems([]byte(tc.data), tc.key)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, got, tc.wantLen)
		})
	}
}

func TestMergePages(T *testing.T) {
	T.Parallel()

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

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var pages [][]byte
			for _, p := range tc.pages {
				pages = append(pages, []byte(p))
			}

			got, err := mergePages(pages, tc.arrayKey)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Compare as JSON to ignore whitespace differences
			var gotJSON, wantJSON interface{}
			json.Unmarshal(got, &gotJSON)
			json.Unmarshal([]byte(tc.want), &wantJSON)

			assert.Equal(t, wantJSON, gotJSON)
		})
	}
}

func TestFetchAllPages(T *testing.T) {
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
	require.NoError(T, err, "Failed to get client")

	pages, err := fetchAllPages(client, "/app/rest/builds", nil)
	require.NoError(T, err)
	assert.Len(T, pages, 3, "fetchAllPages() page count")

	// Verify we can extract items from all pages
	arrayKey, _ := detectArrayKey(pages[0])
	merged, err := mergePages(pages, arrayKey)
	require.NoError(T, err)

	var items []map[string]int
	json.Unmarshal(merged, &items)
	assert.Len(T, items, 5, "merged result item count")
}

func TestFetchAllPagesSinglePage(T *testing.T) {
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
	require.NoError(T, err, "Failed to get client")

	pages, err := fetchAllPages(client, "/app/rest/builds", nil)
	require.NoError(T, err)
	assert.Len(T, pages, 1, "fetchAllPages() page count")
}
