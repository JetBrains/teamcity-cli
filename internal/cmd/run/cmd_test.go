package run_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/config"
)

func init() { color.NoColor = true }

const (
	testJob     = "TestProject_Build"
	testBuildID = "1"
)

func TestRunList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "list", "--limit", "5")
	cmdtest.RunCmdWithFactory(T, f, "run", "list", "--favorites", "--limit", "5")
	cmdtest.RunCmdWithFactory(T, f, "run", "list", "--user", "@me", "--limit", "1")
	cmdtest.RunCmdWithFactory(T, f, "run", "list", "--job", testJob, "--limit", "3")
	cmdtest.RunCmdWithFactory(T, f, "run", "list", "--project", "TestProject", "--status", "success", "--limit", "2")
	cmdtest.RunCmdWithFactory(T, f, "run", "list", "--json", "--limit", "2")
}

func TestRunListBackwardsDateRange(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactoryExpectErr(T, ts.Factory, "is more recent than", "run", "list", "--since", "2020-01-01", "--until", "2019-01-01")
}

func TestRunView(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "view", testBuildID)
	cmdtest.RunCmdWithFactory(T, f, "run", "view", testBuildID, "--json")
}

func TestRunStart(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "start", testJob, "--comment", "CLI test")
}

func TestRunStartWithOptions(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "start", testJob,
		"-P", "key1=val1",
		"-S", "sys.prop=sysval",
		"-E", "ENV_VAR=envval",
		"-m", "Full options test",
		"-t", "test-tag",
		"--clean",
	)
}

func TestRunStartDryRun(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	got := cmdtest.CaptureOutput(T, ts.Factory, "run", "start", testJob, "--dry-run")
	assert.Contains(T, got, "Would trigger run for")
	assert.Contains(T, got, testJob)
}

func TestRunStartReuseDepsDryRun(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	ts.Handle("GET /app/rest/builds/id:6946", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Build{ID: 6946, Number: "42", Status: "SUCCESS", BuildTypeID: "Dep_A"})
	})
	ts.Handle("GET /app/rest/builds/id:6917", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Build{ID: 6917, Number: "41", Status: "SUCCESS", BuildTypeID: "Dep_B"})
	})
	got := cmdtest.CaptureOutput(T, ts.Factory, "run", "start", testJob,
		"--reuse-deps", "6946,6917", "--dry-run")
	assert.Contains(T, got, "Snapshot dependencies:")
	assert.Contains(T, got, "6946")
	assert.Contains(T, got, "#42")
	assert.Contains(T, got, "Dep_A")
	assert.Contains(T, got, "6917")
}

func TestRunStartReuseDepsSendsIDs(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	var captured api.TriggerBuildRequest
	ts.Handle("POST /app/rest/buildQueue", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(T, json.Unmarshal(body, &captured))
		cmdtest.JSON(w, api.Build{ID: 999, BuildTypeID: testJob, WebURL: "https://example/build/999"})
	})

	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "start", testJob, "--reuse-deps", "6946,6917")

	require.NotNil(T, captured.SnapshotDependencies)
	require.Len(T, captured.SnapshotDependencies.Build, 2)
	assert.Equal(T, 6946, captured.SnapshotDependencies.Build[0].ID)
	assert.Equal(T, 6917, captured.SnapshotDependencies.Build[1].ID)
}

func TestRunStartDryRunNonExistentJob(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	err := cmdtest.CaptureErr(T, ts.Factory, "run", "start", "NonExistentJob123456", "--dry-run")
	assert.Contains(T, err.Error(), "not found")
}

func TestRunCancel(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "cancel", testBuildID, "--comment", "Test cleanup")
}

func TestRunLog(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "log", testBuildID)
}

func TestRunLogJSON(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	got := cmdtest.CaptureOutput(T, ts.Factory, "run", "log", testBuildID, "--json")
	assert.Contains(T, got, `"run_id"`)
	assert.Contains(T, got, `"log"`)
	assert.Contains(T, got, "Build started")
}

func TestRunLogJSON_failed(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	ts.Handle("GET /app/rest/builds/id:1", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Build{
			ID:     1,
			Number: "1",
			Status: "FAILURE",
			State:  "finished",
			WebURL: ts.URL + "/viewLog.html?buildId=1",
		})
	})
	got := cmdtest.CaptureOutput(T, ts.Factory, "run", "log", testBuildID, "--json", "--failed")
	assert.Contains(T, got, `"run_id"`)
	assert.Contains(T, got, `"status"`)
	assert.Contains(T, got, `"problems"`)
	assert.Contains(T, got, "FAILURE")
}

func TestRunLogJSON_raw_mutually_exclusive(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	err := cmdtest.CaptureErr(T, ts.Factory, "run", "log", testBuildID, "--json", "--raw")
	assert.Contains(T, err.Error(), "if any flags in the group [json raw] are set none of the others can be")
}

func TestRunLogJSON_job(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	got := cmdtest.CaptureOutput(T, ts.Factory, "run", "log", "--job", testJob, "--json")
	assert.Contains(T, got, `"run_id"`)
	assert.Contains(T, got, `"log"`)
}

func TestRunLogTail(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	got := cmdtest.CaptureOutput(T, f, "run", "log", testBuildID, "--tail", "10")
	assert.Contains(T, got, "Build started")
	assert.Contains(T, got, "Build finished")

	got = cmdtest.CaptureOutput(T, f, "run", "log", testBuildID, "--tail", "10", "--json")
	assert.Contains(T, got, `"messages"`)
}

func TestRunLogFollow(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	ts.Handle("GET /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Build{
			ID:     1,
			Number: "1",
			Status: "SUCCESS",
			State:  "finished",
			WebURL: ts.URL + "/viewLog.html?buildId=1",
		})
	})
	got := cmdtest.CaptureOutput(T, ts.Factory, "run", "log", testBuildID, "--follow")
	assert.Contains(T, got, "Build started")
	assert.Contains(T, got, "Build finished")
}

func TestRunArtifacts(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "artifacts", testBuildID)
	cmdtest.RunCmdWithFactory(T, f, "run", "artifacts", testBuildID, "--json")
	cmdtest.RunCmdWithFactory(T, f, "run", "artifacts", "--job", testJob)
	cmdtest.RunCmdWithFactory(T, f, "run", "artifacts", testBuildID, "--path", "logs", "--json")
	cmdtest.RunCmdWithFactoryExpectErr(T, f, "failed to get artifacts", "run", "artifacts", testBuildID, "--path", "nonexistent")
}

func TestRunPinUnpin(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "pin", testBuildID, "--comment", "CLI test pin")
	cmdtest.RunCmdWithFactory(T, f, "run", "unpin", testBuildID)
}

func TestRunTagUntag(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "tag", testBuildID, "cli-test-tag", "another-tag")
	cmdtest.RunCmdWithFactory(T, f, "run", "untag", testBuildID, "cli-test-tag", "another-tag")
}

func TestRunComment(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "comment", testBuildID, "CLI test comment")
	cmdtest.RunCmdWithFactory(T, f, "run", "comment", testBuildID)
	cmdtest.RunCmdWithFactory(T, f, "run", "comment", testBuildID, "--delete")
}

func TestRunChanges(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "changes", testBuildID)
	cmdtest.RunCmdWithFactory(T, f, "run", "changes", testBuildID, "--no-files")
	cmdtest.RunCmdWithFactory(T, f, "run", "changes", testBuildID, "--json")
}

func TestRunTree(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "tree", testBuildID)
	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "tree", testBuildID, "--depth", "2")
}

func TestRunTreeWithDeps(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	ts.Handle("GET /app/rest/builds", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "snapshotDependency") {
			cmdtest.JSON(w, api.BuildList{
				Count: 1,
				Builds: []api.Build{
					{
						ID:          2,
						Number:      "2",
						Status:      "SUCCESS",
						State:       "finished",
						BuildTypeID: "TestProject_UnitTests",
						BuildType:   &api.BuildType{ID: "TestProject_UnitTests", Name: "Unit Tests"},
					},
				},
			})
			return
		}
		cmdtest.JSON(w, api.BuildList{Count: 0, Builds: []api.Build{}})
	})

	got := cmdtest.CaptureOutput(T, ts.Factory, "run", "tree", testBuildID)
	assert.Contains(T, got, "Unit Tests")
}

func TestRunTests(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "run", "tests", testBuildID)
	cmdtest.RunCmdWithFactory(T, f, "run", "tests", testBuildID, "--failed")
	cmdtest.RunCmdWithFactory(T, f, "run", "tests", testBuildID, "--json")
}

func TestRunListWithAtMe(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	config.SetUserForServer("http://mock.teamcity.test", "admin")
	cmdtest.RunCmdWithFactory(T, ts.Factory, "run", "list", "--user", "@me", "--limit", "5")
}

func TestInvalidStatusFilter(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	rootCmd := cmd.NewRootCmdWithFactory(ts.Factory)
	rootCmd.SetArgs([]string{"run", "list", "--status", "invalid"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	assert.Error(T, err, "expected error for invalid status")
	assert.Contains(T, err.Error(), "invalid status")
}

func TestValidStatusFilter(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	validStatuses := []string{"success", "failure", "running", "queued", "canceled"}
	for _, status := range validStatuses {
		T.Run(status, func(t *testing.T) {
			rootCmd := cmd.NewRootCmdWithFactory(ts.Factory)
			rootCmd.SetArgs([]string{"run", "list", "--status", status, "--limit", "1"})
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			require.NoError(t, err, "expected no error for valid status %s", status)
		})
	}
}

func TestStatusFilterLocator(T *testing.T) {
	tests := []struct {
		status        string
		wantLocator   string // substring that must appear in the locator query
		wantState     string // additional substring that must appear (e.g., state:finished)
		rejectLocator string // substring that must NOT appear
	}{
		{"success", "status%3ASUCCESS", "state%3Afinished", ""},
		{"failure", "status%3AFAILURE", "state%3Afinished", ""},
		{"running", "state%3Arunning", "", "status%3ARUNNING"},
		{"queued", "state%3Aqueued", "", "status%3AQUEUED"},
		{"error", "status%3AERROR", "state%3Afinished", ""},
		{"unknown", "status%3AUNKNOWN", "state%3Afinished", ""},
		{"canceled", "status%3AUNKNOWN", "state%3Afinished", ""},
	}

	for _, tt := range tests {
		T.Run(tt.status, func(t *testing.T) {
			var capturedQuery string
			ts := cmdtest.NewTestServer(t)
			ts.Handle("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
				cmdtest.JSON(w, api.Server{VersionMajor: 2025, VersionMinor: 7, BuildNumber: "197398"})
			})
			ts.Handle("HEAD /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			ts.Handle("GET /app/rest/builds", func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.RawQuery
				cmdtest.JSON(w, api.BuildList{Count: 0, Builds: []api.Build{}})
			})

			rootCmd := cmd.NewRootCmdWithFactory(ts.Factory)
			rootCmd.SetArgs([]string{"run", "list", "--status", tt.status, "--limit", "1"})
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			require.NoError(t, err)

			assert.Contains(t, capturedQuery, tt.wantLocator,
				"--status %s: expected locator to contain %s, got query: %s", tt.status, tt.wantLocator, capturedQuery)
			if tt.wantState != "" {
				assert.Contains(t, capturedQuery, tt.wantState,
					"--status %s: expected locator to contain %s, got query: %s", tt.status, tt.wantState, capturedQuery)
			}
			if tt.rejectLocator != "" {
				assert.NotContains(t, capturedQuery, tt.rejectLocator,
					"--status %s: locator must not contain %s, got query: %s", tt.status, tt.rejectLocator, capturedQuery)
			}
		})
	}
}

func TestRunListFavoritesLocator(T *testing.T) {
	var capturedQuery string
	ts := cmdtest.NewTestServer(T)
	ts.Handle("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Server{VersionMajor: 2025, VersionMinor: 7, BuildNumber: "197398"})
	})
	ts.Handle("HEAD /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts.Handle("GET /app/rest/builds", func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		cmdtest.JSON(w, api.BuildList{Count: 0, Builds: []api.Build{}})
	})

	rootCmd := cmd.NewRootCmdWithFactory(ts.Factory)
	rootCmd.SetArgs([]string{"run", "list", "--favorites", "--limit", "1"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(T, err)

	assert.Contains(T, capturedQuery, api.BuildsOptions{Favorites: true}.Locator().Encode())
	assert.Contains(T, capturedQuery, "count%3A1")
}

func TestRunList_plain(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "list", "--plain")
	want := "" +
		"STATUS \tID\tJOB              \tBRANCH\tTRIGGERED_BY\tDURATION\tAGE   \n" +
		"success\t1 \tTestProject_Build\t-     \t-           \t1m 0s   \tJan 01\n"
	assert.Equal(t, want, got)
}

func TestRunView_output(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/rest/builds/id:42", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Build{
			ID:          42,
			Number:      "7",
			Status:      "SUCCESS",
			State:       "finished",
			StatusText:  "Tests passed: 128",
			BuildTypeID: "TestProject_Build",
			BuildType:   &api.BuildType{ID: "TestProject_Build", Name: "Build"},
			BranchName:  "main",
			StartDate:   "20240101T120000+0000",
			FinishDate:  "20240101T120130+0000",
			WebURL:      "https://ci.example.com/viewLog.html?buildId=42",
			Triggered:   &api.Triggered{Type: "user", User: &api.User{Name: "Alice"}},
			Agent:       &api.Agent{ID: 1, Name: "Agent-Linux-01"},
			Tags:        &api.TagList{Tag: []api.Tag{{Name: "release"}, {Name: "v2.0"}}},
		})
	})
	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "view", "42")
	want := cmdtest.Dedent(`
		✓ Build 42  #7 · main
		Triggered by Alice · Jan 01 · Took 1m 30s

		Status: Tests passed: 128

		Agent: Agent-Linux-01

		Tags: release, v2.0

		View in browser: https://ci.example.com/viewLog.html?buildId=42
	`)
	assert.Equal(t, want, got)
}

func TestRunView_usedByOtherBuilds(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/rest/builds/id:55", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Build{
			ID:                55,
			Number:            "10",
			Status:            "SUCCESS",
			State:             "finished",
			BuildTypeID:       "TestProject_Build",
			BuildType:         &api.BuildType{ID: "TestProject_Build", Name: "Build"},
			BranchName:        "main",
			StartDate:         "20240101T120000+0000",
			FinishDate:        "20240101T120000+0000",
			WebURL:            "https://ci.example.com/viewLog.html?buildId=55",
			Triggered:         &api.Triggered{Type: "snapshotDependency"},
			UsedByOtherBuilds: true,
		})
	})
	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "view", "55")
	assert.Contains(t, got, "Results shared in build chain")
}

func TestRunView_waitReason(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/rest/builds/id:60", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Build{
			ID:          60,
			Number:      "11",
			Status:      "",
			State:       "queued",
			BuildTypeID: "TestProject_Build",
			BuildType:   &api.BuildType{ID: "TestProject_Build", Name: "Build"},
			BranchName:  "main",
			WebURL:      "https://ci.example.com/viewLog.html?buildId=60",
			Triggered:   &api.Triggered{Type: "user", User: &api.User{Name: "Bob"}},
			WaitReason:  "No compatible agents available",
		})
	})
	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "view", "60")
	assert.Contains(t, got, "Wait reason: No compatible agents available")
}

func TestRunStart_reused(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("POST /app/rest/buildQueue", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Build{
			ID:          42,
			Number:      "7",
			State:       "finished",
			Status:      "SUCCESS",
			BuildTypeID: "TestProject_Build",
			WebURL:      ts.URL + "/viewLog.html?buildId=42",
		})
	})
	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "start", testJob)
	assert.Contains(t, got, "Reused existing")
	assert.Contains(t, got, "(optimization)")
}

func TestRunList_invalid_status(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	err := cmdtest.CaptureErr(t, ts.Factory, "run", "list", "--status", "bogus")
	assert.Equal(t, `invalid status "bogus", must be one of: success, failure, running, queued, error, unknown, canceled`, err.Error())
}

func TestRunList_invalid_limit(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	err := cmdtest.CaptureErr(t, ts.Factory, "run", "list", "--limit", "0")
	assert.Equal(t, "--limit must be a positive number, got 0", err.Error())
}
