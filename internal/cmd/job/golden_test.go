package job_test

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func init() { color.NoColor = true }

func TestJobList_plain(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "job", "list", "--plain")
	want := "" +
		"ID               \tNAME \tPROJECT\tSTATUS\n" +
		"TestProject_Build\tBuild\t       \tActive\n"
	assert.Equal(t, want, got)
}

func TestJobList_plain_no_header(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "job", "list", "--plain", "--no-header")
	want := "" +
		"TestProject_Build\tBuild\t       \tActive\n"
	assert.Equal(t, want, got)
}

func TestJobView_output(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "job", "view", testJob)
	want := cmdtest.Dedent(`
		Build
		ID: TestProject_Build
		Project:  (TestProject)
		Status: Active

		View in browser: ` + ts.URL + `/viewType.html?buildTypeId=TestProject_Build
	`)
	assert.Equal(t, want, got)
}
