package pool_test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func TestPoolList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "pool", "list")
	cmdtest.RunCmdWithFactory(T, f, "pool", "list", "--json")
}

func TestPoolView(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "pool", "view", "0")
	cmdtest.RunCmdWithFactory(T, f, "pool", "view", "0", "--json")
}

func TestPoolLinkUnlink(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "pool", "link", "1", "TestProject")
	cmdtest.RunCmdWithFactory(T, f, "pool", "unlink", "1", "TestProject")
}
