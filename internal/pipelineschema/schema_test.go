package pipelineschema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostedAgentNames(t *testing.T) {
	t.Parallel()

	// Shape of the 2026.2 server schema: hosted enum first, then self-hosted const and object forms.
	server := []byte(`{"definitions": {"runOn": {"anyOf": [
		{"type": "string", "title": "JetBrains-hosted agents", "enum": ["Windows-Small", "Mac-Medium", "Linux-Large"]},
		{"type": "string", "title": "Self-hosted agents", "enum": ["self-hosted"]},
		{"type": "object", "title": "Self-hosted agents"}
	]}}}`)
	assert.Equal(t, []string{"Windows-Small", "Mac-Medium", "Linux-Large"}, HostedAgentNames(server))

	// Self-hosted-only enums never count as hosted agent names.
	selfOnly := []byte(`{"definitions": {"runOn": {"anyOf": [{"type": "string", "enum": ["self-hosted"]}]}}}`)
	assert.Nil(t, HostedAgentNames(selfOnly))

	assert.Nil(t, HostedAgentNames(Bytes), "embedded schema does not constrain agent names")
	assert.Nil(t, HostedAgentNames(nil))
	assert.Nil(t, HostedAgentNames([]byte("not json")))
}

func TestConvertYAMLToJSON(t *testing.T) {
	t.Parallel()

	// jsonschema only accepts string keys, so map[any]any keys must stringify.
	assert.Equal(t,
		map[string]any{"1": "one", "two": 2},
		ConvertYAMLToJSON(map[any]any{1: "one", "two": 2}))

	in := map[string]any{
		"jobs": map[any]any{
			"build": map[string]any{
				"steps": []any{
					map[any]any{"script": "go build"},
				},
			},
		},
	}
	got := ConvertYAMLToJSON(in).(map[string]any)
	jobs := got["jobs"].(map[string]any)
	build := jobs["build"].(map[string]any)
	steps := build["steps"].([]any)
	assert.Equal(t, "go build", steps[0].(map[string]any)["script"])
}
