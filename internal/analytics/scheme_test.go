package analytics

import (
	"testing"

	fus "github.com/JetBrains/fus-reporting-api-go"
)

// TestScheme_BuildsValidator ensures every group/event/field in the scheme is
// well-formed enough for fus.NewValidator to accept. A failure here usually
// means a typo in a rule expression or a duplicated reference name.
func TestScheme_BuildsValidator(t *testing.T) {
	if _, err := fus.NewValidator(Scheme); err != nil {
		t.Fatalf("fus.NewValidator(Scheme): %v", err)
	}
}

func TestScheme_HasAllExpectedGroups(t *testing.T) {
	want := map[string]bool{
		GroupSession: false, GroupCommand: false, GroupAPI: false,
		GroupAuth: false, GroupBuild: false,
		GroupAgent: false, GroupPipeline: false, GroupSkill: false,
	}
	for _, g := range Scheme.Groups {
		if _, ok := want[g.ID]; ok {
			want[g.ID] = true
		} else {
			t.Errorf("scheme contains unexpected group %q", g.ID)
		}
	}
	for id, present := range want {
		if !present {
			t.Errorf("scheme missing expected group %q", id)
		}
	}
}

// TestScheme_FlatGroupsRespectFieldCap covers groups where every declared field
// is sent on every event (session, command, api). Multi-event groups (build,
// auth, agent, etc.) declare a union of fields across events, so they are
// excluded — Track() enforces the per-event 10-field cap at runtime.
func TestScheme_FlatGroupsRespectFieldCap(t *testing.T) {
	flat := map[string]bool{
		GroupSession: true,
		GroupCommand: true,
		GroupAPI:     true,
	}
	for _, g := range Scheme.Groups {
		if !flat[g.ID] || g.Rules == nil {
			continue
		}
		if n := len(g.Rules.EventData); n > fus.MaxDataFields {
			t.Errorf("flat group %q declares %d fields, exceeds FUS cap of %d", g.ID, n, fus.MaxDataFields)
		}
	}
}

func TestNormalizeCommand(t *testing.T) {
	if got := NormalizeCommand("run.start"); got != "run.start" {
		t.Errorf("known command: got %q, want run.start", got)
	}
	if got := NormalizeCommand("never.heard.of"); got != "other" {
		t.Errorf("unknown command: got %q, want other", got)
	}
}

func TestNormalizeAIAgent(t *testing.T) {
	cases := map[string]string{
		"":               "none",
		"claude-code":    "claude_code",
		"junie":          "junie",
		"github-copilot": "github_copilot",
		"some-new-thing": "other",
	}
	for in, want := range cases {
		if got := NormalizeAIAgent(in); got != want {
			t.Errorf("NormalizeAIAgent(%q) = %q, want %q", in, got, want)
		}
	}
}
