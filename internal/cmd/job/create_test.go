package job_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func handleBuildTypeCreate(ts *cmdtest.TestServer, captured *[]byte) {
	ts.Handle("POST /app/rest/buildTypes", func(w http.ResponseWriter, r *http.Request) {
		if captured != nil {
			*captured, _ = io.ReadAll(r.Body)
		}
		cmdtest.JSON(w, api.BuildType{
			ID:          "MyProject_Build",
			Name:        "Build",
			ProjectName: "MyProject",
			ProjectID:   "MyProject",
			WebURL:      "https://tc.example.com/buildConfiguration/MyProject_Build",
		})
	})
}

func TestJobCreate(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	handleBuildTypeCreate(ts, nil)

	out := cmdtest.CaptureOutput(T, ts.Factory, "job", "create", "Build", "--project", "MyProject")
	assert.Contains(T, out, "Created job")
	assert.Contains(T, out, `in project "MyProject"`)
}

func TestJobCreateWithID(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	var captured []byte
	ts.Handle("POST /app/rest/buildTypes", func(w http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
		cmdtest.JSON(w, api.BuildType{ID: "Custom", Name: "Build", ProjectID: "MyProject"})
	})

	out := cmdtest.CaptureOutput(T, ts.Factory, "job", "create", "Build", "--project", "MyProject", "--id", "Custom")
	assert.Contains(T, out, "id: Custom")

	var payload map[string]any
	require.NoError(T, json.Unmarshal(captured, &payload))
	assert.Equal(T, "Custom", payload["id"])
	assert.Equal(T, "Build", payload["name"])
	project, ok := payload["project"].(map[string]any)
	require.True(T, ok, "project should be an object")
	assert.Equal(T, "MyProject", project["id"])
}

func TestJobCreateWithTemplate(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	var captured []byte
	handleBuildTypeCreate(ts, &captured)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "job", "create", "Build", "--project", "MyProject", "--template", "MyTemplate")

	var payload struct {
		Templates struct {
			BuildType []struct {
				ID string `json:"id"`
			} `json:"buildType"`
		} `json:"templates"`
	}
	require.NoError(T, json.Unmarshal(captured, &payload))
	require.Len(T, payload.Templates.BuildType, 1)
	assert.Equal(T, "MyTemplate", payload.Templates.BuildType[0].ID)
}

func TestJobCreateJSON(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	handleBuildTypeCreate(ts, nil)

	out := cmdtest.CaptureOutput(T, ts.Factory, "job", "create", "Build", "--project", "MyProject", "--json")
	assert.Contains(T, out, `"id"`)
	assert.Contains(T, out, `"webUrl"`)
}

func TestJobCreateMissingProject(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	T.Setenv("TEAMCITY_PROJECT", "")

	cmdtest.RunCmdWithFactoryExpectErr(T, ts.Factory, "project id is required", "job", "create", "Build")
}

func TestJobCreateServerError(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	ts.Handle("POST /app/rest/buildTypes", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.Error(w, http.StatusForbidden, "Access denied")
	})

	cmdtest.RunCmdWithFactoryExpectErr(T, ts.Factory, "failed to create job", "job", "create", "Build", "--project", "MyProject")
}
