package queue_test

import (
	"net/http"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func init() { color.NoColor = true }

func setupQueueWithItems(t *testing.T) *cmdtest.TestServer {
	t.Helper()
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/rest/buildQueue", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildQueue{
			Count: 2,
			Builds: []api.QueuedBuild{
				{ID: 100, BuildTypeID: "Project_Build", BranchName: "main", State: "queued"},
				{ID: 101, BuildTypeID: "Project_Test", BranchName: "feature/cli", State: "queued"},
			},
		})
	})
	return ts
}

func TestQueueList_plain(t *testing.T) {
	ts := setupQueueWithItems(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "queue", "list", "--plain")
	want := "" +
		"ID \tJOB          \tBRANCH     \tSTATE \n" +
		"100\tProject_Build\tmain       \tqueued\n" +
		"101\tProject_Test \tfeature/cli\tqueued\n"
	assert.Equal(t, want, got)
}

func TestQueueList_plain_no_header(t *testing.T) {
	ts := setupQueueWithItems(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "queue", "list", "--plain", "--no-header")
	want := "" +
		"100\tProject_Build\tmain       \tqueued\n" +
		"101\tProject_Test \tfeature/cli\tqueued\n"
	assert.Equal(t, want, got)
}

func TestQueueList_empty(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "queue", "list")
	assert.Equal(t, "No runs in queue\n", got)
}
