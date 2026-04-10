package run_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/config"
)

func setupDiffServer(t *testing.T) *cmdtest.TestServer {
	t.Helper()
	ts := cmdtest.NewTestServer(t)

	ts.Handle("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Server{VersionMajor: 2025, VersionMinor: 7, BuildNumber: "197398"})
	})
	ts.Handle("HEAD /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("GET /app/rest/builds/id:1", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/resulting-properties") {
			cmdtest.JSON(w, api.ParameterList{
				Count: 2,
				Property: []api.Parameter{
					{Name: "version", Value: "1.0.0"},
					{Name: "env.JAVA_HOME", Value: "/usr/lib/jvm/java-11"},
				},
			})
			return
		}
		cmdtest.JSON(w, api.Build{
			ID:          1,
			Number:      "1",
			Status:      "SUCCESS",
			State:       "finished",
			BuildTypeID: "TestProject_Build",
			BuildType:   &api.BuildType{ID: "TestProject_Build", Name: "Build"},
			BranchName:  "main",
			StartDate:   "20240101T120000+0000",
			FinishDate:  "20240101T120230+0000",
			WebURL:      ts.URL + "/viewLog.html?buildId=1",
			Agent:       &api.Agent{ID: 1, Name: "Agent-Linux-01"},
		})
	})

	ts.Handle("GET /app/rest/builds/id:2", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/resulting-properties") {
			cmdtest.JSON(w, api.ParameterList{
				Count: 3,
				Property: []api.Parameter{
					{Name: "version", Value: "1.0.1"},
					{Name: "env.JAVA_HOME", Value: "/usr/lib/jvm/java-17"},
					{Name: "new.feature", Value: "enabled"},
				},
			})
			return
		}
		cmdtest.JSON(w, api.Build{
			ID:          2,
			Number:      "2",
			Status:      "FAILURE",
			State:       "finished",
			BuildTypeID: "TestProject_Build",
			BuildType:   &api.BuildType{ID: "TestProject_Build", Name: "Build"},
			BranchName:  "main",
			StartDate:   "20240101T130000+0000",
			FinishDate:  "20240101T130315+0000",
			StatusText:  "Tests failed: 2",
			WebURL:      ts.URL + "/viewLog.html?buildId=2",
			Agent:       &api.Agent{ID: 2, Name: "Agent-Linux-02"},
		})
	})

	ts.Handle("GET /app/rest/changes", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		if strings.Contains(q, "id%3A1") || strings.Contains(q, "id:1") {
			cmdtest.JSON(w, api.ChangeList{
				Count: 1,
				Change: []api.Change{
					{ID: 1, Version: "aaa1111", Username: "alice", Comment: "Initial commit"},
				},
			})
			return
		}
		if strings.Contains(q, "id%3A2") || strings.Contains(q, "id:2") {
			cmdtest.JSON(w, api.ChangeList{
				Count: 2,
				Change: []api.Change{
					{ID: 1, Version: "aaa1111", Username: "alice", Comment: "Initial commit"},
					{ID: 2, Version: "bbb2222", Username: "bob", Comment: "Break the tests"},
				},
			})
			return
		}
		cmdtest.JSON(w, api.ChangeList{Count: 0})
	})

	ts.Handle("GET /app/rest/testOccurrences", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		if strings.Contains(q, "id%3A1") || strings.Contains(q, "id:1") {
			cmdtest.JSON(w, api.TestOccurrences{
				Count:  3,
				Passed: 3,
				Failed: 0,
				TestOccurrence: []api.TestOccurrence{
					{ID: "t1", Name: "TestLogin", Status: "SUCCESS"},
					{ID: "t2", Name: "TestPayment", Status: "SUCCESS"},
					{ID: "t3", Name: "TestSearch", Status: "SUCCESS"},
				},
			})
			return
		}
		if strings.Contains(q, "id%3A2") || strings.Contains(q, "id:2") {
			cmdtest.JSON(w, api.TestOccurrences{
				Count:  3,
				Passed: 1,
				Failed: 2,
				TestOccurrence: []api.TestOccurrence{
					{ID: "t1", Name: "TestLogin", Status: "FAILURE"},
					{ID: "t2", Name: "TestPayment", Status: "FAILURE"},
					{ID: "t3", Name: "TestSearch", Status: "SUCCESS"},
				},
			})
			return
		}
		cmdtest.JSON(w, api.TestOccurrences{Count: 0})
	})

	ts.Handle("GET /app/rest/problemOccurrences", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		if strings.Contains(q, "id%3A1") || strings.Contains(q, "id:1") {
			cmdtest.JSON(w, api.ProblemOccurrences{Count: 0, ProblemOccurrence: []api.ProblemOccurrence{}})
			return
		}
		if strings.Contains(q, "id%3A2") || strings.Contains(q, "id:2") {
			cmdtest.JSON(w, api.ProblemOccurrences{
				Count: 1,
				ProblemOccurrence: []api.ProblemOccurrence{
					{ID: "p1", Type: "TC_FAILED_TESTS", Identity: "failedTests", Details: "2 tests failed"},
				},
			})
			return
		}
		cmdtest.JSON(w, api.ProblemOccurrences{Count: 0, ProblemOccurrence: []api.ProblemOccurrence{}})
	})

	ts.Handle("GET /downloadBuildLog.html", func(w http.ResponseWriter, r *http.Request) {
		buildID := r.URL.Query().Get("buildId")
		header := "Build #1 header\nTriggered\nStarted on agent\nFinished\nVCS rev\nTeamCity URL\nServer version\nServer timezone\n"
		switch buildID {
		case "1":
			cmdtest.Text(w, header+"[12:00:01] Compiling...\n[12:00:05] Running tests...\n[12:00:08] TestLogin passed\n[12:00:09] TestPayment passed\n[12:00:10] All tests passed\n[12:00:10] Build finished")
		case "2":
			cmdtest.Text(w, header+"[13:00:01] Compiling...\n[13:00:05] Running tests...\n[13:00:08]E: TestLogin FAILED\n[13:00:09]E: TestPayment FAILED\n[13:00:10] 2 tests failed\n[13:00:10] Build finished with errors")
		default:
			cmdtest.Text(w, "")
		}
	})

	config.SetUserForServer(ts.URL, "admin")
	return ts
}

func TestRunDiffLog(t *testing.T) {
	ts := setupDiffServer(t)

	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "diff", "1", "2", "--log")

	assert.Contains(t, got, "#1")
	assert.Contains(t, got, "#2")
	assert.Contains(t, got, "@@")
	assert.Contains(t, got, "-")
	assert.Contains(t, got, "All tests passed")
	assert.Contains(t, got, "+")
	assert.Contains(t, got, "TestLogin FAILED")
	assert.Contains(t, got, "TestPayment FAILED")
}

func TestRunDiffLogIdentical(t *testing.T) {
	ts := cmdtest.NewTestServer(t)
	ts.Handle("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Server{VersionMajor: 2025, VersionMinor: 7, BuildNumber: "197398"})
	})
	ts.Handle("HEAD /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts.Handle("GET /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Build{ID: 1, Number: "1", Status: "SUCCESS", State: "finished"})
	})
	ts.Handle("GET /downloadBuildLog.html", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.Text(w, "h1\nh2\nh3\nh4\nh5\nh6\nh7\nh8\n[12:00:00] Build started\n[12:00:10] Build finished")
	})
	config.SetUserForServer(ts.URL, "admin")

	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "diff", "1", "1", "--log")
	assert.Contains(t, got, "identical")
}

func TestRunDiff(t *testing.T) {
	ts := setupDiffServer(t)

	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "diff", "1", "2")

	assert.Contains(t, got, "COMPARING")
	assert.Contains(t, got, "#1")
	assert.Contains(t, got, "#2")

	assert.Contains(t, got, "STATUS")
	assert.Contains(t, got, "Success")
	assert.Contains(t, got, "Failed")

	assert.Contains(t, got, "DURATION")

	assert.Contains(t, got, "AGENT")
	assert.Contains(t, got, "Agent-Linux-01")
	assert.Contains(t, got, "Agent-Linux-02")

	assert.Contains(t, got, "PARAMETERS")
	assert.Contains(t, got, "version")
	assert.Contains(t, got, "1.0.0")
	assert.Contains(t, got, "1.0.1")
	assert.Contains(t, got, "new.feature")

	assert.Contains(t, got, "CHANGES")
	assert.Contains(t, got, "bbb2222")
	assert.Contains(t, got, "bob")

	assert.Contains(t, got, "TESTS")
	assert.Contains(t, got, "TestLogin")
	assert.Contains(t, got, "TestPayment")
	assert.Contains(t, got, "New failures")

	assert.Contains(t, got, "PROBLEMS")
	assert.Contains(t, got, "2 tests failed")

	assert.Contains(t, got, "Changed:")
	assert.Contains(t, got, "View -")
	assert.Contains(t, got, "View +")
}

func TestRunDiffJSON(t *testing.T) {
	ts := setupDiffServer(t)

	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "diff", "1", "2", "--json")

	assert.Contains(t, got, `"run1"`)
	assert.Contains(t, got, `"run2"`)
	assert.Contains(t, got, `"diff"`)
	assert.Contains(t, got, `"status"`)
	assert.Contains(t, got, `"parameters"`)
	assert.Contains(t, got, `"tests"`)
	assert.Contains(t, got, `"newFailures"`)
}

func TestRunDiffSingleArg(t *testing.T) {
	ts := setupDiffServer(t)

	ts.Handle("GET /app/rest/builds", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildList{
			Count: 2,
			Builds: []api.Build{
				{ID: 2, Number: "2", Status: "FAILURE", State: "finished", BuildTypeID: "TestProject_Build"},
				{ID: 1, Number: "1", Status: "SUCCESS", State: "finished", BuildTypeID: "TestProject_Build"},
			},
		})
	})

	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "diff", "2")

	assert.Contains(t, got, "COMPARING")
	assert.Contains(t, got, "#1")
	assert.Contains(t, got, "#2")
}

func TestRunDiffIdenticalBuilds(t *testing.T) {
	ts := cmdtest.NewTestServer(t)

	ts.Handle("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Server{VersionMajor: 2025, VersionMinor: 7, BuildNumber: "197398"})
	})
	ts.Handle("HEAD /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	build := api.Build{
		ID: 1, Number: "1", Status: "SUCCESS", State: "finished",
		BuildTypeID: "TestProject_Build",
		BuildType:   &api.BuildType{ID: "TestProject_Build", Name: "Build"},
		StartDate:   "20240101T120000+0000",
		FinishDate:  "20240101T120100+0000",
		Agent:       &api.Agent{ID: 1, Name: "Agent-01"},
	}

	ts.Handle("GET /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/resulting-properties") {
			cmdtest.JSON(w, api.ParameterList{Count: 0})
			return
		}
		cmdtest.JSON(w, build)
	})
	ts.Handle("GET /app/rest/testOccurrences", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.TestOccurrences{Count: 1, Passed: 1, TestOccurrence: []api.TestOccurrence{
			{ID: "t1", Name: "Test1", Status: "SUCCESS"},
		}})
	})
	ts.Handle("GET /app/rest/changes", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.ChangeList{Count: 0})
	})
	ts.Handle("GET /app/rest/problemOccurrences", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.ProblemOccurrences{Count: 0, ProblemOccurrence: []api.ProblemOccurrence{}})
	})

	config.SetUserForServer(ts.URL, "admin")

	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "diff", "1", "1")
	assert.Contains(t, got, "No differences found")
}
