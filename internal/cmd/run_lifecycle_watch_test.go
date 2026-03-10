package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
)

func TestDoRunWatchLogsFallsBackWithoutTTY(t *testing.T) {
	t.Helper()

	var buildRequests int
	var logRequests int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/app/rest/builds/id:123":
			buildRequests++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(api.Build{
				ID:          123,
				Number:      "42",
				BuildTypeID: "Test_Build",
				WebURL:      "https://example.invalid/build/123",
				State:       "finished",
				Status:      "SUCCESS",
			})
			return
		case r.Method == http.MethodGet && r.URL.Path == "/downloadBuildLog.html":
			logRequests++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(""))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	origGetClient := GetClientFunc
	origRunWatchTUIFn := runWatchTUIFn
	origWatchHasTTYFn := watchHasTTYFn
	t.Cleanup(func() {
		GetClientFunc = origGetClient
		runWatchTUIFn = origRunWatchTUIFn
		watchHasTTYFn = origWatchHasTTYFn
	})

	GetClientFunc = func() (api.ClientInterface, error) {
		return api.NewClient(ts.URL, "test-token"), nil
	}

	tuiCalled := false
	runWatchTUIFn = func(client api.ClientInterface, runID string, interval int) error {
		tuiCalled = true
		return errors.New("runWatchTUI should not be called without TTY")
	}
	watchHasTTYFn = func() bool { return false }

	err := doRunWatch("123", &runWatchOptions{interval: 1, logs: true})
	if err != nil {
		t.Fatalf("doRunWatch returned error: %v", err)
	}
	if tuiCalled {
		t.Fatal("runWatchTUI was called without TTY")
	}
	if buildRequests < 2 {
		t.Fatalf("expected at least 2 build requests, got %d", buildRequests)
	}
	if logRequests != 0 {
		t.Fatalf("expected 0 build log requests in fallback mode, got %d", logRequests)
	}
}

func TestDoRunWatchLogsUsesTUIWhenTTYIsAvailable(t *testing.T) {
	t.Helper()

	origGetClient := GetClientFunc
	origRunWatchTUIFn := runWatchTUIFn
	origWatchHasTTYFn := watchHasTTYFn
	t.Cleanup(func() {
		GetClientFunc = origGetClient
		runWatchTUIFn = origRunWatchTUIFn
		watchHasTTYFn = origWatchHasTTYFn
	})

	GetClientFunc = func() (api.ClientInterface, error) {
		return api.NewClient("https://example.invalid", "test-token"), nil
	}

	sentinelErr := errors.New("tui path reached")
	tuiCalled := false
	runWatchTUIFn = func(client api.ClientInterface, runID string, interval int) error {
		tuiCalled = true
		if runID != "123" {
			t.Fatalf("unexpected runID: %s", runID)
		}
		if interval != 7 {
			t.Fatalf("unexpected interval: %d", interval)
		}
		return sentinelErr
	}
	watchHasTTYFn = func() bool { return true }

	err := doRunWatch("123", &runWatchOptions{interval: 7, logs: true})
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected sentinel error, got: %v", err)
	}
	if !tuiCalled {
		t.Fatal("expected runWatchTUI to be called when TTY is available")
	}
}
