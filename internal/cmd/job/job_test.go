package job_test

import (
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
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
