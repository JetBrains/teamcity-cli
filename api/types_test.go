package api

import (
	"encoding/json"
	"testing"
)

func TestMuteJSONRoundTrip(t *testing.T) {
	// Payload modeled on TeamCity's /app/rest/mutes representation.
	raw := `{
		"id": 42,
		"scope": {
			"buildTypes": {
				"count": 1,
				"buildType": [{"id": "Project_Build", "name": "Build"}]
			}
		},
		"target": {
			"tests": {
				"count": 1,
				"test": [{"id": "-1234567890", "name": "com.example.FooTest.bar"}]
			}
		},
		"resolution": {"type": "manually"},
		"assignment": {"text": "flaky on CI", "timestamp": "20260604T120000+0000"},
		"mutedTime": "20260604T120000+0000"
	}`

	var m Mute
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal Mute: %v", err)
	}

	if m.ID != 42 {
		t.Errorf("ID = %d, want 42", m.ID)
	}
	if m.Scope == nil || m.Scope.BuildTypes == nil || len(m.Scope.BuildTypes.BuildType) != 1 {
		t.Fatalf("scope build types not parsed: %+v", m.Scope)
	}
	if got := m.Scope.BuildTypes.BuildType[0].ID; got != "Project_Build" {
		t.Errorf("scope buildType id = %q, want Project_Build", got)
	}
	if m.Target == nil || m.Target.Tests == nil || len(m.Target.Tests.Test) != 1 {
		t.Fatalf("target tests not parsed: %+v", m.Target)
	}
	if got := m.Target.Tests.Test[0].Name; got != "com.example.FooTest.bar" {
		t.Errorf("target test name = %q", got)
	}
	if m.Resolution == nil || m.Resolution.Type != "manually" {
		t.Errorf("resolution = %+v, want type manually", m.Resolution)
	}
	if m.Assignment == nil || m.Assignment.Text != "flaky on CI" {
		t.Errorf("assignment text = %+v", m.Assignment)
	}
	if m.MutedTime != "20260604T120000+0000" {
		t.Errorf("mutedTime = %q", m.MutedTime)
	}

	// Re-marshal and ensure it round-trips back into an equivalent struct.
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal Mute: %v", err)
	}
	var m2 Mute
	if err := json.Unmarshal(out, &m2); err != nil {
		t.Fatalf("re-unmarshal Mute: %v", err)
	}
	if m2.ID != m.ID || m2.Resolution.Type != m.Resolution.Type ||
		m2.Target.Tests.Test[0].ID != m.Target.Tests.Test[0].ID {
		t.Errorf("round-trip mismatch: %+v vs %+v", m, m2)
	}
}

func TestInvestigationJSONRoundTrip(t *testing.T) {
	// Payload modeled on TeamCity's /app/rest/investigations representation.
	raw := `{
		"id": "test_123",
		"state": "TAKEN",
		"scope": {
			"project": {"id": "Sandbox", "name": "Sandbox"}
		},
		"target": {
			"tests": {
				"count": 1,
				"test": [{"id": "-987", "name": "com.example.FooTest.bar"}]
			}
		},
		"assignee": {"username": "jdoe", "name": "Jane Doe"},
		"resolution": {"type": "whenFixed"}
	}`

	var inv Investigation
	if err := json.Unmarshal([]byte(raw), &inv); err != nil {
		t.Fatalf("unmarshal Investigation: %v", err)
	}

	if inv.ID != "test_123" {
		t.Errorf("ID = %q, want test_123", inv.ID)
	}
	if inv.State != "TAKEN" {
		t.Errorf("State = %q, want TAKEN", inv.State)
	}
	if inv.Scope == nil || inv.Scope.Project == nil || inv.Scope.Project.ID != "Sandbox" {
		t.Fatalf("scope project not parsed: %+v", inv.Scope)
	}
	if inv.Assignee == nil || inv.Assignee.Username != "jdoe" {
		t.Errorf("assignee = %+v, want username jdoe", inv.Assignee)
	}
	if inv.Resolution == nil || inv.Resolution.Type != "whenFixed" {
		t.Errorf("resolution = %+v", inv.Resolution)
	}

	out, err := json.Marshal(inv)
	if err != nil {
		t.Fatalf("marshal Investigation: %v", err)
	}
	var inv2 Investigation
	if err := json.Unmarshal(out, &inv2); err != nil {
		t.Fatalf("re-unmarshal Investigation: %v", err)
	}
	if inv2.State != inv.State || inv2.Assignee.Username != inv.Assignee.Username ||
		inv2.Target.Tests.Test[0].Name != inv.Target.Tests.Test[0].Name {
		t.Errorf("round-trip mismatch: %+v vs %+v", inv, inv2)
	}
}

func TestMutesListJSONRoundTrip(t *testing.T) {
	raw := `{"count": 2, "mute": [{"id": 1}, {"id": 2}]}`
	var ms Mutes
	if err := json.Unmarshal([]byte(raw), &ms); err != nil {
		t.Fatalf("unmarshal Mutes: %v", err)
	}
	if ms.Count != 2 || len(ms.Mute) != 2 || ms.Mute[1].ID != 2 {
		t.Errorf("Mutes not parsed: %+v", ms)
	}

	raw = `{"count": 1, "investigation": [{"id": "x", "state": "FIXED"}]}`
	var is Investigations
	if err := json.Unmarshal([]byte(raw), &is); err != nil {
		t.Fatalf("unmarshal Investigations: %v", err)
	}
	if is.Count != 1 || len(is.Investigation) != 1 || is.Investigation[0].State != "FIXED" {
		t.Errorf("Investigations not parsed: %+v", is)
	}
}
