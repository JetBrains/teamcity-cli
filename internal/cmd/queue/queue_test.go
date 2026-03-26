package queue_test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func TestQueueList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "queue", "list", "--limit", "10")
	cmdtest.RunCmdWithFactory(T, f, "queue", "list", "--job", "TestProject_Build")
	cmdtest.RunCmdWithFactory(T, f, "queue", "list", "--json")
}

func TestQueueRemove(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "queue", "remove", "100")
}

func TestQueueTop(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "queue", "top", "100")
}
