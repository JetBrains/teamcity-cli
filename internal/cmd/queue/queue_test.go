package queue_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

func TestQueueRemove(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "queue", "remove", "100")
}

func TestQueueTop(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "queue", "top", "100")
}
