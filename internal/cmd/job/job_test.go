package job_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/output"
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

func TestJobTreeDepth(T *testing.T) {
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

	ts.Handle("GET /app/rest/buildTypes/id:Deploy/dependents", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildTypeList{Count: 0})
	})

	ts.Handle("GET /app/rest/buildTypes/id:Deploy", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildType{ID: "Deploy", Name: "Deploy", ProjectID: "MyProject"})
	})

	cmdtest.RunCmdWithFactory(T, ts.CloneFactory(), "job", "tree", "Deploy", "--only", "dependencies", "--depth", "1")
	cmdtest.RunCmdWithFactory(T, ts.CloneFactory(), "job", "tree", "Deploy", "--only", "dependencies", "--depth", "2")
}

func TestJobTreeDepthOutput(T *testing.T) {
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

	runWithBuf := func(args ...string) string {
		T.Helper()
		var buf bytes.Buffer
		f := ts.CloneFactory()
		f.Printer = &output.Printer{Out: &buf, ErrOut: &buf}
		cmdtest.RunCmdWithFactory(T, f, args...)
		return buf.String()
	}

	// --depth 1 shows Deploy and its direct dependency (Build), but not transitive (Compile)
	out1 := runWithBuf("job", "tree", "Deploy", "--only", "dependencies", "--depth", "1")
	require.Contains(T, out1, "Build", "--depth 1 should show direct dependencies")
	require.NotContains(T, out1, "Compile", "--depth 1 should not show transitive dependencies")

	// --depth 2 shows Deploy, Build, and Compile
	out2 := runWithBuf("job", "tree", "Deploy", "--only", "dependencies", "--depth", "2")
	require.Contains(T, out2, "Build", "--depth 2 should show direct dependencies")
	require.Contains(T, out2, "Compile", "--depth 2 should show transitive dependencies")
}
