package analytics

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSession_FreshCreatesNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".session")
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	s, err := loadOrCreateSessionAt(path, now)
	if err != nil {
		t.Fatalf("loadOrCreateSession: %v", err)
	}
	if !s.IsNew {
		t.Error("expected IsNew=true on cold start")
	}
	if s.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if !s.LastActive.Equal(now) {
		t.Errorf("LastActive = %v, want %v", s.LastActive, now)
	}
}

func TestSession_ReusedWithinWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".session")
	t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	first, err := loadOrCreateSessionAt(path, t0)
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	// 10 minutes later — well inside the 30-min window.
	t1 := t0.Add(10 * time.Minute)
	second, err := loadOrCreateSessionAt(path, t1)
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("session ID changed within window: %q -> %q", first.ID, second.ID)
	}
	if second.IsNew {
		t.Error("second invocation in same window should not be marked new")
	}
	if !second.LastActive.Equal(t1) {
		t.Errorf("LastActive not bumped: got %v, want %v", second.LastActive, t1)
	}
}

func TestSession_RotatesAfterWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".session")
	t0 := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	first, err := loadOrCreateSessionAt(path, t0)
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	// Exactly the inactivity window — should rotate (>= comparison).
	t1 := t0.Add(SessionWindow)
	second, err := loadOrCreateSessionAt(path, t1)
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if second.ID == first.ID {
		t.Error("session ID should rotate after inactivity window")
	}
	if !second.IsNew {
		t.Error("rotated session should be marked new")
	}
}

func TestNewSessionID_Format(t *testing.T) {
	for range 5 {
		id, err := newSessionID()
		if err != nil {
			t.Fatalf("newSessionID: %v", err)
		}
		if len(id) != 36 {
			t.Errorf("id length = %d, want 36 (got %q)", len(id), id)
		}
		if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
			t.Errorf("id %q missing dashes", id)
		}
		// version 4
		if id[14] != '4' {
			t.Errorf("id %q version nibble = %c, want 4", id, id[14])
		}
	}
}
