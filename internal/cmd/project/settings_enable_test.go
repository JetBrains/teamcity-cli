package project_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
)

func TestSettingsEnable(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/rest/projects/TestProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"synchronizationMode":"disabled"}`) })
	ts.Handle("PUT /app/rest/projects/id:TestProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "importFromVCS", body["importDecision"])
		assert.Equal(t, true, body["allowUIEditing"])
		assert.Equal(t, true, body["storeSecureValuesOutsideVcs"])
		assert.Equal(t, "Settings", body["vcsRootId"])
		assert.Equal(t, "enabled", body["synchronizationMode"])
		fmt.Fprint(w, `{"format":"kotlin","synchronizationMode":"enabled"}`)
	})
	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "settings", "enable", "TestProject", "--vcs-root", "Settings", "--json")
	assert.Contains(t, out, `"synchronizationMode": "enabled"`)
	assert.NotContains(t, out, "import requested")
}

func TestSettingsEnablePreservesExistingConfiguration(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("PUT /app/rest/projects/id:TestProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) { t.Error("must not modify existing settings") })
	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "already has versioned settings", "project", "settings", "enable", "TestProject", "--vcs-root", "Settings")
}
