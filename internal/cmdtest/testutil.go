// Package cmdtest provides shared test helpers for CLI command tests.
package cmdtest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer wraps httptest.Server for easy API testing.
type TestServer struct {
	*httptest.Server
	Factory  *cmdutil.Factory
	handlers map[string]http.HandlerFunc
	t        *testing.T
}

// NewTestServer creates a test server and configures a Factory with a mock client.
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

		t.Logf("Unhandled request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))

	t.Setenv("TEAMCITY_URL", ts.URL)
	t.Setenv("TEAMCITY_TOKEN", "test-token")
	t.Setenv("TC_INSECURE_SKIP_WARN", "1")
	_ = config.Init()

	ts.Factory = cmdutil.NewFactory()
	ts.Factory.ClientFunc = func() (api.ClientInterface, error) {
		return api.NewClient(ts.URL, "test-token"), nil
	}

	t.Cleanup(func() {
		ts.Close()
	})

	return ts
}

// CloneFactory returns a new Factory that shares the same ClientFunc and IOStreams
// but has its own flag storage, making it safe for parallel subtests.
func (ts *TestServer) CloneFactory() *cmdutil.Factory {
	return &cmdutil.Factory{
		IOStreams:   ts.Factory.IOStreams,
		ClientFunc:  ts.Factory.ClientFunc,
	}
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

// ExtractID extracts an ID from a path like /app/rest/builds/id:123/something
func ExtractID(path, prefix string) string {
	_, after, ok := strings.Cut(path, prefix)
	if !ok {
		return ""
	}
	rest := after
	end := strings.IndexAny(rest, "/?")
	if end == -1 {
		return rest
	}
	return rest[:end]
}

// RunCmd executes a CLI command using the default mock factory and asserts no error.
func RunCmd(t *testing.T, args ...string) {
	t.Helper()
	rootCmd := cmd.NewRootCmd()
	rootCmd.SetArgs(args)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err, "Execute(%v)", args)
}

// RunCmdWithFactory executes a CLI command using a specific factory and asserts no error.
func RunCmdWithFactory(t *testing.T, f *cmdutil.Factory, args ...string) {
	t.Helper()
	rootCmd := cmd.NewRootCmdWithFactory(f)
	rootCmd.SetArgs(args)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err, "Execute(%v)", args)
}

// RunCmdExpectErr executes a CLI command and asserts an error containing want.
func RunCmdExpectErr(t *testing.T, want string, args ...string) {
	t.Helper()
	rootCmd := cmd.NewRootCmd()
	rootCmd.SetArgs(args)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.Error(t, err, "expected error for Execute(%v)", args)
	assert.Contains(t, err.Error(), want)
}

// RunCmdWithFactoryExpectErr executes a CLI command using a specific factory and asserts an error containing want.
func RunCmdWithFactoryExpectErr(t *testing.T, f *cmdutil.Factory, want string, args ...string) {
	t.Helper()
	rootCmd := cmd.NewRootCmdWithFactory(f)
	rootCmd.SetArgs(args)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.Error(t, err, "expected error for Execute(%v)", args)
	assert.Contains(t, err.Error(), want)
}
