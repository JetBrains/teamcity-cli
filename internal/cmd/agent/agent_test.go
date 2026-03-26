package agent_test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func TestAgentList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "agent", "list")
	cmdtest.RunCmdWithFactory(T, f, "agent", "list", "--pool", "Default")
	cmdtest.RunCmdWithFactory(T, f, "agent", "list", "--connected")
	cmdtest.RunCmdWithFactory(T, f, "agent", "list", "--enabled")
	cmdtest.RunCmdWithFactory(T, f, "agent", "list", "--authorized")
	cmdtest.RunCmdWithFactory(T, f, "agent", "list", "--json")
	cmdtest.RunCmdWithFactory(T, f, "agent", "list", "--limit", "10")
}

func TestAgentView(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "agent", "view", "1")
	cmdtest.RunCmdWithFactory(T, f, "agent", "view", "Agent 1", "--json")
}

func TestAgentEnableDisable(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "agent", "enable", "1")
	cmdtest.RunCmdWithFactory(T, f, "agent", "disable", "Agent 1")
}

func TestAgentAuthorize(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "agent", "authorize", "1")
	cmdtest.RunCmdWithFactory(T, f, "agent", "deauthorize", "Agent 1")
}

func TestAgentJobs(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "agent", "jobs", "1")
	cmdtest.RunCmdWithFactory(T, f, "agent", "jobs", "Agent 1", "--json")
	cmdtest.RunCmdWithFactory(T, f, "agent", "jobs", "1", "--incompatible")
	cmdtest.RunCmdWithFactory(T, f, "agent", "jobs", "1", "--incompatible", "--json")
}

func TestAgentMove(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactory(T, ts.Factory, "agent", "move", "Agent 1", "0")
}

func TestAgentReboot(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "agent", "reboot", "Agent 1")
	cmdtest.RunCmdWithFactory(T, f, "agent", "reboot", "1", "--after-build")
}
