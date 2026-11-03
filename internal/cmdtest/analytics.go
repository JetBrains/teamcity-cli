package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	fus "github.com/JetBrains/fus-reporting-api-go"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/require"
)

// EnableAnalytics undoes NewTestServer's DO_NOT_TRACK for tests of opt-out-gated behavior.
// The CLI config moves to a fresh XDG_CONFIG_HOME, returned, and FUS events go to a local sink, so nothing leaves the machine.
func EnableAnalytics(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(analytics.EnvDoNotTrack, "")
	t.Setenv(analytics.EnvAnalytics, "")
	config.ResetForTest()
	require.NoError(t, config.Init())
	require.NoError(t, config.MarkAnalyticsNoticeShown())

	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(sink.Close)
	dataDir, err := analytics.DataDir()
	require.NoError(t, err)
	// A fresh cached FUS config keeps boot off the network; no metadata endpoint means the embedded scheme.
	seed, err := json.Marshal(&fus.FUSConfig{SendEndpoint: sink.URL, FetchedAt: time.Now().Unix()})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "fus_config.json"), seed, 0o600))
	return dir
}

// OnboardingCall is one request to an /app/fus/onboarding/cli/ endpoint; Body is "" when it had none.
type OnboardingCall struct {
	Endpoint string
	Body     string
}

// OnboardingCalls records the onboarding endpoint calls a TestServer gets.
type OnboardingCalls struct {
	mu     sync.Mutex
	status int
	calls  []OnboardingCall
}

// HandleOnboarding answers every onboarding endpoint with status and records the calls.
func (ts *TestServer) HandleOnboarding(status int) *OnboardingCalls {
	rec := &OnboardingCalls{status: status}
	ts.Handle("POST /app/fus/onboarding/cli/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.calls = append(rec.calls, OnboardingCall{Endpoint: strings.TrimPrefix(r.URL.Path, "/app/fus/onboarding/cli/"), Body: string(body)})
		status := rec.status
		rec.mu.Unlock()
		w.WriteHeader(status)
	})
	return rec
}

// Respond changes the status the onboarding endpoints answer with.
func (c *OnboardingCalls) Respond(status int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

// Calls returns the calls so far.
func (c *OnboardingCalls) Calls() []OnboardingCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls)
}

// Reset forgets the calls so far.
func (c *OnboardingCalls) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = nil
}
