package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdatePipelineYAML verifies the update body carries both request shapes:
// the flat top-level fields read by pre-2026.2 servers and the nested "pipeline"
// object read by 2026.2+, each preserving the existing settings.
func TestUpdatePipelineYAML(t *testing.T) {
	t.Parallel()

	const newYAML = "jobs:\n  build:\n    steps: []\n"
	var body map[string]any

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/app/pipeline/CLI_CiCd", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name":     "CI/CD",
				"yaml":     "old",
				"vcsRoot":  map[string]any{"id": "MyRepo"},
				"triggers": []any{map[string]any{"type": "vcs"}},
			})
		case http.MethodPost:
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &body))
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	require.NoError(t, client.UpdatePipelineYAML("CLI_CiCd", newYAML))

	// Top-level shape for pre-2026.2 servers.
	assert.Equal(t, "CI/CD", body["name"])
	assert.Equal(t, newYAML, body["yaml"])
	assert.Equal(t, map[string]any{"externalVcsRootId": "MyRepo"}, body["vcsRoot"])
	assert.Contains(t, body, "triggers")

	// Nested shape for 2026.2+ servers, carrying the same fields.
	nested, ok := body["pipeline"].(map[string]any)
	require.True(t, ok, "body must contain a nested pipeline object")
	assert.Equal(t, "CI/CD", nested["name"])
	assert.Equal(t, newYAML, nested["yaml"])
	assert.Equal(t, map[string]any{"externalVcsRootId": "MyRepo"}, nested["vcsRoot"])
	assert.Contains(t, nested, "triggers")
}
