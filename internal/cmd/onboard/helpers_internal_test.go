package onboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/require"
)

// newTestClient serves routes ("METHOD /path" patterns) plus a 2025.7 /server, failing the test on any other request.
func newTestClient(t *testing.T, routes map[string]http.HandlerFunc) *api.Client {
	t.Helper()
	mux := http.NewServeMux()
	if _, ok := routes["GET /app/rest/server"]; !ok {
		mux.HandleFunc("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, api.Server{VersionMajor: 2025, VersionMinor: 7})
		})
	}
	for pattern, h := range routes {
		mux.HandleFunc(pattern, h)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api.NewClient(srv.URL, "test-token")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(t *testing.T, r *http.Request, v any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(r.Body).Decode(v))
}

// testFactory returns a non-interactive Factory whose output goes to the returned buffer.
func testFactory() (*cmdutil.Factory, *bytes.Buffer) {
	var buf bytes.Buffer
	return &cmdutil.Factory{NoInput: true, Printer: &output.Printer{Out: &buf, ErrOut: &buf}}, &buf
}

// initRepoWithCommit creates a local git repository on branch with one commit, usable as a remote URL offline.
func initRepoWithCommit(t *testing.T, branch string) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"checkout", "-b", branch},
		{"-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	return dir
}

func property(props []api.Property, name string) string {
	for _, p := range props {
		if p.Name == name {
			return p.Value
		}
	}
	return ""
}
