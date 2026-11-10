package onboard

import (
	"bytes"
	"cmp"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/onboarding"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient serves routes ("METHOD /path" patterns) plus a 2025.7 /server and the onboarding endpoints, failing the test on any other request.
func newTestClient(t *testing.T, routes map[string]http.HandlerFunc) *api.Client {
	t.Helper()
	client, _ := newRecordingClient(t, routes)
	return client
}

// onboardingCall is one request to an /app/fus/onboarding/cli/ endpoint; body is "" when it had none.
type onboardingCall struct {
	endpoint string
	body     string
}

// onboardingRecorder records the onboarding endpoint calls and answers them with status (204 by default).
type onboardingRecorder struct {
	mu     sync.Mutex
	status int
	calls  []onboardingCall
}

func (r *onboardingRecorder) record(c onboardingCall) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, c)
	return cmp.Or(r.status, http.StatusNoContent)
}

// respond sets the status the onboarding endpoints answer with.
func (r *onboardingRecorder) respond(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = status
}

// endpoints returns the called endpoint names in order.
func (r *onboardingRecorder) endpoints() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []string{}
	for _, c := range r.calls {
		out = append(out, c.endpoint)
	}
	return out
}

// body returns the body of the only call to endpoint, failing the test unless there is exactly one.
func (r *onboardingRecorder) body(t *testing.T, endpoint string) string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	var bodies []string
	for _, c := range r.calls {
		if c.endpoint == endpoint {
			bodies = append(bodies, c.body)
		}
	}
	require.Len(t, bodies, 1, "calls to %s", endpoint)
	return bodies[0]
}

// newRecordingClient is newTestClient that also returns the recorder of onboarding endpoint calls.
// It clears the analytics opt-outs, since the onboarding endpoints aren't called without analytics,
// and sends background reports synchronously, so each is recorded when the call that sends it returns.
func newRecordingClient(t *testing.T, routes map[string]http.HandlerFunc) (*api.Client, *onboardingRecorder) {
	t.Helper()
	t.Setenv(analytics.EnvDoNotTrack, "")
	t.Setenv(analytics.EnvAnalytics, "")
	orig := notifyInBackground
	notifyInBackground = onboarding.Notify
	t.Cleanup(func() { notifyInBackground = orig })
	rec := &onboardingRecorder{}
	mux := http.NewServeMux()
	if _, ok := routes["GET /app/rest/server"]; !ok {
		mux.HandleFunc("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, api.Server{VersionMajor: 2025, VersionMinor: 7})
		})
	}
	mux.HandleFunc("POST /app/fus/onboarding/cli/{endpoint}", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		w.WriteHeader(rec.record(onboardingCall{endpoint: r.PathValue("endpoint"), body: string(body)}))
	})
	for pattern, h := range routes {
		mux.HandleFunc(pattern, h)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api.NewClient(srv.URL, "test-token"), rec
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
