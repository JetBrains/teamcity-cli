package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestRootCmd() *cobra.Command {
	return createTestRootCmdWithFactory(cmdutil.NewFactory())
}

func createTestRootCmdWithFactory(f *cmdutil.Factory) *cobra.Command {
	rootCmd := &cobra.Command{
		Use: "teamcity",
	}
	rootCmd.PersistentFlags().Bool("no-color", false, "")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "")
	rootCmd.PersistentFlags().Bool("verbose", false, "")
	rootCmd.PersistentFlags().Bool("no-input", false, "")
	rootCmd.AddCommand(NewCmd(f))
	return rootCmd
}

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
		json.NewEncoder(w).Encode(map[string]any{
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

		var body map[string]any
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

func TestAPICommandXMLErrorResponse(T *testing.T) {
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`<errors><error><message>Field 'snapshot-dependencies' is not supported. Supported are: number, status, statusText, id, startDate, finishDate, buildTypeId, branchName.</message><additionalMessage>jetbrains.buildServer.server.rest.errors.NotFoundException</additionalMessage><statusText>Responding with error, status code: 404 (Not Found).</statusText></error></errors>`))
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds/id:999/snapshot-dependencies"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.Error(T, err, "expected error for 404 response")
	assert.Contains(T, err.Error(), "404")
}

func TestAPICommand406RetriesWithWildcardAccept(T *testing.T) {
	requestCount := 0
	setupMockServerForAPI(T, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		accept := r.Header.Get("Accept")
		if accept == "application/json" {
			w.WriteHeader(http.StatusNotAcceptable)
			w.Write([]byte(`{"errors":[{"message":"Make sure you have supplied correct 'Accept' header."}]}`))
			return
		}
		// Retry with */* gets the real XML error
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`<errors><error><message>Field 'foo' is not supported.</message></error></errors>`))
	})

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds/id:1/foo"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	require.Error(T, err)
	assert.Contains(T, err.Error(), "404")
	assert.Equal(T, 2, requestCount)
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
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		buildType, ok := body["buildType"].(map[string]any)
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

	f := cmdutil.NewFactory()
	f.IOStreams.In = strings.NewReader(`{"test":"stdin"}`)

	var out bytes.Buffer
	rootCmd := createTestRootCmdWithFactory(f)
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
			json.NewEncoder(w).Encode(map[string]any{
				"count":    2,
				"nextHref": "/app/rest/builds?start=2",
				"build":    []map[string]int{{"id": 1}, {"id": 2}},
			})
		case 2:
			json.NewEncoder(w).Encode(map[string]any{
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
		json.NewEncoder(w).Encode(map[string]any{
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
			json.NewEncoder(w).Encode(map[string]any{
				"count":    2,
				"nextHref": "/app/rest/builds?start=2",
				"build":    []map[string]int{{"id": 1}, {"id": 2}},
			})
		case 2:
			json.NewEncoder(w).Encode(map[string]any{
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

func TestExtractNextHref(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{"has nextHref", `{"count":100,"nextHref":"/app/rest/builds?start=100","build":[]}`, "/app/rest/builds?start=100", false},
		{"no nextHref", `{"count":50,"build":[]}`, "", false},
		{"empty nextHref", `{"count":50,"nextHref":"","build":[]}`, "", false},
		{"invalid json", `not json`, "", true},
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
		{"builds response", `{"count":2,"build":[{"id":1},{"id":2}]}`, "build", false},
		{"buildTypes response", `{"count":2,"buildType":[{"id":"bt1"},{"id":"bt2"}]}`, "buildType", false},
		{"projects response", `{"count":2,"project":[{"id":"p1"},{"id":"p2"}]}`, "project", false},
		{"agents response", `{"count":1,"agent":[{"id":1}]}`, "agent", false},
		{"no array key (single object)", `{"id":1,"name":"test"}`, "", false},
		{"empty array", `{"count":0,"build":[]}`, "build", false},
		{"invalid json", `not json`, "", true},
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
		{"extract builds", `{"count":2,"build":[{"id":1},{"id":2}]}`, "build", 2, false},
		{"key not found", `{"count":0,"build":[]}`, "project", 0, false},
		{"empty array", `{"count":0,"build":[]}`, "build", 0, false},
		{"invalid json", `not json`, "build", 0, true},
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
		{"merge two pages", []string{`{"count":2,"build":[{"id":1},{"id":2}]}`, `{"count":2,"build":[{"id":3},{"id":4}]}`}, "build", `[{"id":1},{"id":2},{"id":3},{"id":4}]`, false},
		{"single page", []string{`{"count":2,"build":[{"id":1},{"id":2}]}`}, "build", `[{"id":1},{"id":2}]`, false},
		{"empty pages", []string{`{"count":0,"build":[]}`, `{"count":0,"build":[]}`}, "build", `[]`, false},
		{"mixed sizes", []string{`{"count":3,"build":[{"id":1},{"id":2},{"id":3}]}`, `{"count":1,"build":[{"id":4}]}`}, "build", `[{"id":1},{"id":2},{"id":3},{"id":4}]`, false},
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
			var gotJSON, wantJSON any
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
			json.NewEncoder(w).Encode(map[string]any{"count": 2, "nextHref": "/app/rest/builds?start=2", "build": []map[string]int{{"id": 1}, {"id": 2}}})
		case 2:
			json.NewEncoder(w).Encode(map[string]any{"count": 2, "nextHref": "/app/rest/builds?start=4", "build": []map[string]int{{"id": 3}, {"id": 4}}})
		case 3:
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "build": []map[string]int{{"id": 5}}})
		}
	}))
	defer server.Close()

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

	client := api.NewClient(server.URL, "test-token")

	pages, err := fetchAllPages(T.Context(), client, "/app/rest/builds", nil)
	require.NoError(T, err)
	assert.Len(T, pages, 3, "fetchAllPages() page count")

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
		json.NewEncoder(w).Encode(map[string]any{"count": 2, "build": []map[string]int{{"id": 1}, {"id": 2}}})
	}))
	defer server.Close()

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

	client := api.NewClient(server.URL, "test-token")

	pages, err := fetchAllPages(T.Context(), client, "/app/rest/builds", nil)
	require.NoError(T, err)
	assert.Len(T, pages, 1, "fetchAllPages() page count")
}

func TestPrettyPrintJSON(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name        string
		body        string
		wantOK      bool
		wantContain string
	}{
		{
			name:        "JSON body",
			body:        `{"errors":[{"message":"some error"}]}`,
			wantOK:      true,
			wantContain: "some error",
		},
		{
			name:        "XML error converted to JSON",
			body:        `<errors><error><message>Field 'snapshot-dependencies' is not supported.</message></error></errors>`,
			wantOK:      true,
			wantContain: "snapshot-dependencies",
		},
		{
			name:        "XML error with declaration",
			body:        `<?xml version="1.0" encoding="UTF-8"?><errors><error><message>Not found</message></error></errors>`,
			wantOK:      true,
			wantContain: "Not found",
		},
		{
			name:   "plain text",
			body:   `Resource not found`,
			wantOK: false,
		},
		{
			name:   "non-error XML",
			body:   `<server><version>2025.7</version></server>`,
			wantOK: false,
		},
		{
			name:   "empty errors element",
			body:   `<errors></errors>`,
			wantOK: false,
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, ok := prettyPrintJSON([]byte(tc.body))
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantContain != "" {
				assert.Contains(t, result, tc.wantContain)
			}
		})
	}
}
