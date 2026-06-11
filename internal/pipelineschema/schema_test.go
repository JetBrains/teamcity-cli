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
