package run_test

import (
	"net/http"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func init() { color.NoColor = true }

func TestRunList_plain(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "list", "--plain")
	want := "" +
		"STATUS \tID\tJOB              \tBRANCH\tTRIGGERED_BY\tDURATION\tAGE   \n" +
		"success\t1 \tTestProject_Build\t-     \t-           \t1m 0s   \tJan 01\n"
	assert.Equal(t, want, got)
}

func TestRunList_plain_no_header(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "list", "--plain", "--no-header")
	want := "" +
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
