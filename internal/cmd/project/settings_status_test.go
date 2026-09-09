package project_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
)

func TestSettingsStatusRuntimeMessage(t *testing.T) {
	for _, message := range []string{"Synchronization is disabled", "Versioned Settings have never been enabled in this project", "Settings are up to date."} {
		t.Run(message, func(t *testing.T) {
			ts := cmdtest.SetupMockClient(t)
			ts.Handle("GET /app/rest/projects/TestProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"synchronizationMode":"enabled","format":"kotlin"}`)
			})
			ts.Handle("GET /app/rest/projects/TestProject/versionedSettings/status", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"type":"info","message":%q,"timestamp":"20260828T180800+0000"}`, message)
			})
			out := cmdtest.CaptureOutput(t, ts.Factory, "project", "settings", "status", "TestProject")
			assert.Contains(t, out, message)
			assert.NotContains(t, out, "synchronized")
			assert.NotContains(t, out, "Last sync")
			assert.Contains(t, out, "Recorded")
		})
	}
}

func TestSettingsStatusDetails(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/rest/projects/TestProject/versionedSettings/config", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	ts.Handle("GET /app/rest/projects/TestProject/versionedSettings/status", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"type":"warn","message":"Cannot load settings","missingContextParameters":["service"],"versionedSettingsError":[{"message":"DSL compilation error","type":"COMPILATION_ERROR","file":"settings.kts","stackTraceLines":["detail"]}]}`)
	})
	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "settings", "status", "TestProject")
	assert.Contains(t, out, "Missing context parameters: service")
	assert.Contains(t, out, "DSL compilation error (settings.kts)")
	out = cmdtest.CaptureOutput(t, ts.Factory, "project", "settings", "status", "TestProject", "--json")
	assert.Contains(t, out, `"missingContextParameters"`)
	assert.Contains(t, out, `"stackTraceLines"`)
}
