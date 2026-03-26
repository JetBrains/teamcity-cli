package run

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
)

func TestDoRunWatchLogsFallsBackWithoutTTY(t *testing.T) {
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

	origRunWatchTUIFn := runWatchTUIFn
	origWatchHasTTYFn := watchHasTTYFn
	t.Cleanup(func() {
		runWatchTUIFn = origRunWatchTUIFn
		watchHasTTYFn = origWatchHasTTYFn
	})

	f := &cmdutil.Factory{

		ClientFunc: func() (api.ClientInterface, error) {
			return api.NewClient(ts.URL, "test-token"), nil
		},
	}

	tuiCalled := false
	runWatchTUIFn = func(client api.ClientInterface, runID string, interval int) error {
		tuiCalled = true
		return errors.New("runWatchTUI should not be called without TTY")
	}
	watchHasTTYFn = func() bool { return false }

	err := doRunWatch(f, "123", &runWatchOptions{interval: 1, logs: true})
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

func TestDoRunWatchJSONOutputsOnCompletion(t *testing.T) {
	pollCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/app/rest/builds/id:456" {
			pollCount++
			w.Header().Set("Content-Type", "application/json")
			build := api.Build{
				ID:          456,
				Number:      "7",
				BuildTypeID: "MyJob",
				WebURL:      "https://example.invalid/build/456",
			}
			if pollCount < 3 {
				build.State = "running"
				build.Status = ""
			} else {
				build.State = "finished"
				build.Status = "SUCCESS"
			}
			_ = json.NewEncoder(w).Encode(build)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	f := &cmdutil.Factory{

		ClientFunc: func() (api.ClientInterface, error) {
			return api.NewClient(ts.URL, "test-token"), nil
		},
	}

	err := doRunWatch(f, "456", &runWatchOptions{interval: 1, json: true})
	if err != nil {
		t.Fatalf("doRunWatch with --json returned error: %v", err)
	}
	if pollCount < 3 {
		t.Fatalf("expected at least 3 polls, got %d", pollCount)
	}
}

func TestDoRunWatchJSONReturnsExitErrorOnFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/app/rest/builds/id:789" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(api.Build{
				ID:          789,
				Number:      "1",
				BuildTypeID: "FailJob",
				WebURL:      "https://example.invalid/build/789",
				State:       "finished",
				Status:      "FAILURE",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	f := &cmdutil.Factory{

		ClientFunc: func() (api.ClientInterface, error) {
			return api.NewClient(ts.URL, "test-token"), nil
		},
	}

	err := doRunWatch(f, "789", &runWatchOptions{interval: 1, json: true})
	if exitErr, ok := errors.AsType[*cmdutil.ExitError](err); !ok {
		t.Fatalf("expected ExitError, got: %v", err)
	} else if exitErr.Code != cmdutil.ExitFailure {
		t.Fatalf("expected exit code %d, got %d", cmdutil.ExitFailure, exitErr.Code)
	}
}

func TestDoRunWatchLogsUsesTUIWhenTTYIsAvailable(t *testing.T) {
	origRunWatchTUIFn := runWatchTUIFn
	origWatchHasTTYFn := watchHasTTYFn
	t.Cleanup(func() {
		runWatchTUIFn = origRunWatchTUIFn
		watchHasTTYFn = origWatchHasTTYFn
	})

	f := &cmdutil.Factory{

		ClientFunc: func() (api.ClientInterface, error) {
			return api.NewClient("https://example.invalid", "test-token"), nil
		},
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

	err := doRunWatch(f, "123", &runWatchOptions{interval: 7, logs: true})
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected sentinel error, got: %v", err)
	}
	if !tuiCalled {
		t.Fatal("expected runWatchTUI to be called when TTY is available")
	}
}
