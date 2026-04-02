package queue_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func TestQueueList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "queue", "list", "--limit", "10")
	cmdtest.RunCmdWithFactory(T, f, "queue", "list", "--job", "TestProject_Build")
	cmdtest.RunCmdWithFactory(T, f, "queue", "list", "--json")
}

func TestQueueList_empty(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "queue", "list")
	assert.Equal(t, "No runs in queue\n", got)
}

func TestQueueList_waitReason(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/rest/buildQueue", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildQueue{
			Count: 2,
			Builds: []api.QueuedBuild{
				{
					ID:          200,
					BuildTypeID: "MyProject_Build",
					State:       "queued",
					BranchName:  "main",
					WaitReason:  "Waiting for build #199 in chain",
				},
				{
					ID:          201,
					BuildTypeID: "MyProject_Test",
					State:       "queued",
					BranchName:  "main",
				},
			},
		})
	})

	got := cmdtest.CaptureOutput(t, ts.Factory, "queue", "list")
	assert.Contains(t, got, "Waiting for build #199 in chain")
	assert.Contains(t, got, "WAIT REASON")
}

func TestQueueRemove(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "queue", "remove", "100")
}

func TestQueueTop(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "queue", "top", "100")
}
