package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBuildTypeSettings(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:MyBuild/settings")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SettingsList{
			Count:    1,
			Property: []Setting{{Name: "buildNumberPattern", Value: "1.0.%build.counter%"}},
		})
	})

	settings, err := client.GetBuildTypeSettings("MyBuild")
	require.NoError(t, err)
	assert.Equal(t, 1, settings.Count)
	assert.Equal(t, "buildNumberPattern", settings.Property[0].Name)
}

func TestGetBuildTypeSetting(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:MyBuild/settings/artifactRules")
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("+:build/** => out\n-:**/*.tmp\n"))
	})

	// Body is returned verbatim so multiline values (artifact/checkout rules)
	// round-trip exactly; display trimming is a presentation concern.
	val, err := client.GetBuildTypeSetting("MyBuild", "artifactRules")
	require.NoError(t, err)
	assert.Equal(t, "+:build/** => out\n-:**/*.tmp\n", val)
}
