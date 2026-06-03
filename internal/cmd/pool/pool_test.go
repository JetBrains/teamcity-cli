package pool_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func TestPoolList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "pool", "list")
	cmdtest.RunCmdWithFactory(T, f, "pool", "list", "--json")
}

func TestPoolListWeb(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "pool", "list", "--web")
	assert.Contains(t, out, ts.URL+"/agents.html?tab=agentPools")
}

func TestPoolView(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "pool", "view", "0")
	cmdtest.RunCmdWithFactory(T, f, "pool", "view", "0", "--json")
}

// --web must validate the pool exists before emitting the URL (not exit 0 on a bad id).
func TestPoolViewWebValidatesPool(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/rest/agentPools/id:", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no pool", http.StatusNotFound)
	})

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "", "pool", "view", "999", "--web")
}

func TestPoolLinkUnlink(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "pool", "link", "1", "TestProject")
	cmdtest.RunCmdWithFactory(T, f, "pool", "unlink", "1", "TestProject")
}
