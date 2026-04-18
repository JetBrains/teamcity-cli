package job_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJob = "TestProject_Build"

func TestJobList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "job", "list", "--limit", "5")
	cmdtest.RunCmdWithFactory(T, f, "job", "list", "--project", "TestProject")
	cmdtest.RunCmdWithFactory(T, f, "job", "list", "--json", "--limit", "2")
}

func TestJobView(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "job", "view", testJob)
	cmdtest.RunCmdWithFactory(T, f, "job", "view", testJob, "--json")
}

func TestJobListContinuePreservesAllMode(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/rest/buildTypes", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "start%3A1") {
			cmdtest.JSON(w, api.BuildTypeList{
				Count: 1,
				Href:  "/app/rest/buildTypes?locator=count:30,start:1",
				BuildTypes: []api.BuildType{
					{ID: "TestProject_CI_Build", Name: "Pipeline Build", ProjectID: "TestProject"},
				},
			})
			return
		}

		cmdtest.JSON(w, api.BuildTypeList{
			Count:    1,
			Href:     "/app/rest/buildTypes?locator=count:1,start:0",
			NextHref: "/app/rest/buildTypes?locator=count:1,start:1",
			BuildTypes: []api.BuildType{
				{ID: "Library_Build", Name: "Library Build", ProjectID: "Library"},
			},
		})
	})

	stdout := cmdtest.CaptureOutput(t, ts.Factory, "job", "list", "--all", "--json", "--limit", "1")

	var firstPage struct {
		Count    int             `json:"count"`
		Items    []api.BuildType `json:"items"`
		Continue string          `json:"continue"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &firstPage))
	require.Len(t, firstPage.Items, 1)
	assert.Equal(t, "Library_Build", firstPage.Items[0].ID)

	path, offset, state, err := cmdutil.DecodeContinueTokenWithState("teamcity job list", firstPage.Continue)
	require.NoError(t, err)
	assert.Equal(t, "/app/rest/buildTypes?locator=count:1,start:1", path)
	assert.Zero(t, offset)
	assert.JSONEq(t, `{"all":true}`, string(state))

	stdout = cmdtest.CaptureOutput(t, ts.Factory, "job", "list", "--continue", firstPage.Continue, "--json")

	var secondPage struct {
		Count    int             `json:"count"`
		Items    []api.BuildType `json:"items"`
		Continue string          `json:"continue"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &secondPage))
	require.Len(t, secondPage.Items, 1)
	assert.Equal(t, "TestProject_CI_Build", secondPage.Items[0].ID)
	assert.Empty(t, secondPage.Continue)
}

func TestJobPauseResume(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "job", "pause", testJob)
	cmdtest.RunCmdWithFactory(T, f, "job", "resume", testJob)
}

func TestJobParam(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	paramName := "TC_CLI_JOB_TEST"

	cmdtest.RunCmdWithFactory(T, f, "job", "param", "list", testJob)
	cmdtest.RunCmdWithFactory(T, f, "job", "param", "set", testJob, paramName, "test_value")
	cmdtest.RunCmdWithFactory(T, f, "job", "param", "get", testJob, paramName)
	cmdtest.RunCmdWithFactory(T, f, "job", "param", "delete", testJob, paramName)
}

func TestJobTree(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "job", "tree", testJob)
}

func TestJobTreeWithDeps(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	ts.Handle("GET /app/rest/buildTypes/id:Deploy/snapshot-dependencies", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.SnapshotDependencyList{
			Count: 1,
			SnapshotDependency: []api.SnapshotDependency{
				{ID: "dep1", SourceBuildType: &api.BuildType{ID: "Build", Name: "Build", ProjectID: "MyProject"}},
			},
		})
	})

	ts.Handle("GET /app/rest/buildTypes/id:Build/snapshot-dependencies", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.SnapshotDependencyList{
			Count: 1,
			SnapshotDependency: []api.SnapshotDependency{
				{ID: "dep2", SourceBuildType: &api.BuildType{ID: "Compile", Name: "Compile", ProjectID: "MyProject"}},
			},
		})
	})

	ts.Handle("GET /app/rest/buildTypes/id:Deploy", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildType{ID: "Deploy", Name: "Deploy", ProjectID: "MyProject"})
	})

	cmdtest.RunCmdWithFactory(T, ts.Factory, "job", "tree", "Deploy")
}
