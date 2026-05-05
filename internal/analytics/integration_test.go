package analytics

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	fus "github.com/JetBrains/fus-reporting-api-go"
)

// TestPipeline_EndToEnd drives the production boot path against a local mock by pre-seeding the FUS config cache, so any regression that breaks boot() (empty salt, missing scheme, validator/anonymizer wiring) fails the test instead of silently no-op'ing in the field.
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
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	// Seed fus_config.json so LoadOrFetchConfig returns our mock send endpoint without hitting the network. MetadataEndpoint is left empty so boot falls back to the embedded Scheme (no CDN call either).
	seed, _ := json.Marshal(&fus.FUSConfig{SendEndpoint: server.URL, FetchedAt: time.Now().Unix()})
	if err := os.WriteFile(filepath.Join(dir, "fus_config.json"), seed, 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	c := New(Config{
		CLIVersion: "0.1.0-test",
		Salt:       Salt,
		Session: &Session{
			ID:         "11111111-2222-4333-8444-555555555555",
			IsNew:      true,
			LastActive: time.Now(),
		},
		Environment: Environment{OS: "darwin", Arch: "arm64", CISystem: CINone, AIAgent: "none"},
		AuthSource:  AuthSourceNone,
	})
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
		t.Fatal("no events reached the wire — boot or flush silently failed")
	}

	var report fus.Report
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("invalid report JSON: %v", err)
	}
	if len(report.Events) != 2 {
		t.Fatalf("expected 2 events (session + command), got %d", len(report.Events))
	}

	var gotSession, gotCommand bool
	for _, e := range report.Events {
		if e.Product != ProductCode {
			t.Errorf("product = %q, want %q", e.Product, ProductCode)
		}
		if e.Recorder.ID != RecorderID {
			t.Errorf("recorder.id = %q, want %q", e.Recorder.ID, RecorderID)
		}
		if e.IDs["device"] == "" {
			t.Error("device id empty (anonymizer didn't run)")
		}
		if e.Session == "" {
			t.Error("wire session empty")
		}
		for k, v := range e.Event.Data {
			if s, ok := v.(string); ok && len(s) >= len("validation.") && s[:len("validation.")] == "validation." {
				t.Errorf("%s/%s field %q was sentinel-replaced: %q", e.Group.ID, e.Event.ID, k, s)
			}
		}
		switch e.Group.ID {
		case GroupSession:
			gotSession = true
			if !e.Group.State {
				t.Error("session event must have state=true")
			}
		case GroupCommand:
			gotCommand = true
			if e.Event.Data["command"] != "run.start" {
				t.Errorf("command field = %v, want run.start", e.Event.Data["command"])
			}
		}
	}
	if !gotSession || !gotCommand {
		t.Errorf("expected both session and command events, got session=%v command=%v", gotSession, gotCommand)
	}
}
