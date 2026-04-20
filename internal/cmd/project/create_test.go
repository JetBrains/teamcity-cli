package project_test

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

func TestProjectCreate(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "create", "MyProject")
	assert.Contains(T, out, "Created project")
	assert.Contains(T, out, "MyProject")
}

func TestProjectCreateWithID(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "create", "My Project", "--id", "CustomID")
	assert.Contains(T, out, "Created project")
	assert.Contains(T, out, "id: CustomID")
}

func TestProjectCreateWithParent(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	var captured []byte
	ts.Handle("POST /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
		cmdtest.JSON(w, api.Project{ID: "Child", Name: "Child", ParentProjectID: "Parent"})
	})

	cmdtest.RunCmdWithFactory(T, ts.Factory, "project", "create", "Child", "--id", "Child", "--parent", "Parent")

	var payload map[string]any
	require.NoError(T, json.Unmarshal(captured, &payload))
	parent, ok := payload["parentProject"].(map[string]any)
	require.True(T, ok, "parentProject should be an object")
	assert.Equal(T, "id:Parent", parent["locator"])
	assert.Equal(T, "Child", payload["id"])
	assert.Equal(T, "Child", payload["name"])
}

func TestProjectCreateJSON(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "create", "MyProject", "--json")
	assert.Contains(T, out, `"id"`)
	assert.Contains(T, out, `"name"`)
}

func TestProjectCreateServerError(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	ts.Handle("POST /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.Error(w, http.StatusForbidden, "Access denied")
	})

	cmdtest.RunCmdWithFactoryExpectErr(T, ts.Factory, "failed to create project", "project", "create", "MyProject")
}
