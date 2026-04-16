package analytics

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	fus "github.com/JetBrains/fus-reporting-api-go"
)

// TestPipeline_EndToEnd exercises the whole client against a local mock that
// stands in for the FUS config + send endpoints. It validates that:
//
//   - boot succeeds when given a fake FUSConfig (no network involved)
//   - TrackSession + TrackCommand both reach the wire
//   - emitted JSON carries the expected product/recorder identity, anonymized
//     device + session, and our group/event/data fields
//
// This is the local equivalent of the staging probe described in DO-A-610 —
// it does not need network access.
func TestPipeline_EndToEnd(t *testing.T) {
	var (
		mu       sync.Mutex
		captured []byte
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		captured = body
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Build a client and override its boot so it uses our mock config.
	c := New(Config{
		CLIVersion: "0.1.0-test",
		Session: &Session{
			ID:         "11111111-2222-4333-8444-555555555555",
			IsNew:      true,
			LastActive: time.Now(),
		},
		Environment: Environment{OS: "darwin", Arch: "arm64", CISystem: CINone, AIAgent: "none"},
		AuthSource:  AuthSourceNone,
	})

	// Inject a logger built with a stubbed FUSConfig so no network is needed.
	validator, err := fus.NewValidator(Scheme)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	logger, err := fus.NewLogger(
		fus.RecorderConfig{
			RecorderID:      RecorderID,
			RecorderVersion: RecorderVersion,
			ProductCode:     ProductCode,
			BuildVersion:    "0.1.0-test",
			DataDir:         t.TempDir(),
			DeviceID:        "test-device",
		},
		fus.WithFUSConfig(&fus.FUSConfig{SendEndpoint: server.URL, Salt: "test-salt"}),
		fus.WithValidator(validator),
	)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	c.logger = logger
	c.bootOnce.Do(func() {})

	c.TrackSession()
	c.TrackCommand(CommandEvent{
		Command:    "run.start",
		HasJSON:    true,
		FlagCount:  2,
		ExitCode:   0,
		DurationMS: 1500,
	})

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	mu.Lock()
	body := captured
	mu.Unlock()
	if body == nil {
		t.Fatal("no events captured at mock send endpoint")
	}

	var report fus.Report
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("invalid report JSON: %v", err)
	}
	if len(report.Events) != 2 {
		t.Fatalf("expected 2 events (session + command), got %d", len(report.Events))
	}

	for _, e := range report.Events {
		if e.Product != ProductCode {
			t.Errorf("product = %q, want %q", e.Product, ProductCode)
		}
		if e.Recorder.ID != RecorderID {
			t.Errorf("recorder.id = %q, want %q", e.Recorder.ID, RecorderID)
		}
		if e.IDs["device"] == "" {
			t.Error("device id empty")
		}
		if e.Session == "" {
			t.Error("wire session empty")
		}
	}

	// Verify each group landed and the command event carries our enums.
	var gotSession, gotCommand bool
	for _, e := range report.Events {
		switch e.Group.ID {
		case GroupSession:
			gotSession = true
			if !e.Group.State {
				t.Error("session event must have state=true")
			}
			if e.Event.Data["ai_agent"] != "none" {
				t.Errorf("session ai_agent = %v, want none", e.Event.Data["ai_agent"])
			}
		case GroupCommand:
			gotCommand = true
			if e.Event.Data["command"] != "run.start" {
				t.Errorf("command field = %v, want run.start", e.Event.Data["command"])
			}
			if e.Event.Data["exit_code"] != "0" {
				t.Errorf("exit_code = %v, want 0", e.Event.Data["exit_code"])
			}
		}
	}
	if !gotSession {
		t.Error("session event missing from captured payload")
	}
	if !gotCommand {
		t.Error("command event missing from captured payload")
	}
}
