package analytics

import (
	"testing"

	fus "github.com/JetBrains/fus-reporting-api-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		GroupWorkspace: false, GroupMigrate: false,
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

func TestDefinitionMatchesEvents(t *testing.T) {
	t.Parallel()
	es, err := Definition.BuildEventsScheme(fus.RecorderConfig{RecorderID: RecorderID, RecorderVersion: RecorderVersion}, "test")
	require.NoError(t, err)
	samples := map[string]map[string]bool{}
	for _, event := range SampleEvents() {
		key := event.Group.ID + "/" + event.Event.ID
		if samples[key] == nil {
			samples[key] = map[string]bool{}
		}
		for field := range event.Event.Data {
			samples[key][field] = true
		}
	}
	for _, group := range es.Scheme {
		assert.Equal(t, groupVersion[group.ID], group.Version)
		for _, event := range group.Schema {
			key := group.ID + "/" + event.Event
			assert.LessOrEqual(t, len(event.Fields), fus.MaxDataFields, key)
			fields := map[string]bool{}
			for _, field := range event.Fields {
				fields[field.Path] = true
				if field.ShouldBeAnonymized {
					assert.Equal(t, "session_id", field.Path)
					assert.Equal(t, []string{"{regexp#hash}"}, field.Value)
				}
			}
			assert.Equal(t, samples[key], fields, key)
			delete(samples, key)
		}
	}
	assert.Empty(t, samples, "sample events missing from the declaration")
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
