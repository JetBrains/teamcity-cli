package analytics

import (
	"testing"
	"time"
)

// TestClient_NilSafe verifies the Track surface accepts a nil receiver — the contract for "disabled".
func TestClient_NilSafe(t *testing.T) {
	var c *Client
	c.TrackSession()
	c.TrackCommand(CommandEvent{Command: "run.start"})
	c.TrackAPI(APIEvent{Method: "GET", Endpoint: "/app/rest/builds"})
	c.Track(GroupAuth, EventLoginCompleted, map[string]any{
		"method": AuthMethodToken, "is_success": true, "error_type": ErrorNone,
	})
	if err := c.Flush(t.Context()); err != nil {
		t.Errorf("Flush nil: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close nil: %v", err)
	}
	if c.SessionID() != "" {
		t.Errorf("nil SessionID = %q, want empty", c.SessionID())
	}
}

// TestClient_BootFailureBecomesNoop exercises the lazy-boot error path.
func TestClient_BootFailureBecomesNoop(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	c := New(Config{
		CLIVersion: "0.1.0-test",
		Session:    &Session{ID: "00000000-0000-4000-8000-000000000000", IsNew: true, LastActive: time.Now()},
		Environment: Environment{
			OS: "darwin", Arch: "arm64", CISystem: CINone, AIAgent: "none",
		},
		AuthSource: AuthSourceNone,
	})

	c.TrackCommand(CommandEvent{Command: "run.list"})
	c.TrackCommand(CommandEvent{Command: "run.list"})
	if err := c.Close(); err != nil {
		t.Errorf("Close after boot failure: %v", err)
	}
}
