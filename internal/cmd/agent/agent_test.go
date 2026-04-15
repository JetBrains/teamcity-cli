package agent_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
)

func init() { color.NoColor = true }

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

func TestAgentList_plain(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "agent", "list", "--plain")
	want := "ID\tNAME   \tPOOL   \tSTATUS      \n" +
		"1 \tAgent 1\tDefault\tConnected   \n" +
		"2 \tAgent 2\tDefault\tDisconnected\n"
	assert.Equal(t, want, got)
}

func TestAgentListPlainPrintsContinuationTokenToStderr(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /app/rest/agents", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.AgentList{
			Count:    2,
			Href:     "/app/rest/agents?locator=count:2,start:0",
			NextHref: "/app/rest/agents?locator=count:2,start:2",
			Agents: []api.Agent{
				{ID: 1, Name: "Agent 1", Connected: true, Enabled: true, Authorized: true, Pool: &api.Pool{Name: "Default"}},
				{ID: 2, Name: "Agent 2", Connected: false, Enabled: true, Authorized: true, Pool: &api.Pool{Name: "Default"}},
			},
		})
	})

	_, stderr := cmdtest.CaptureSplitOutput(t, ts.Factory, "agent", "list", "--plain", "--limit", "2")
	require.Contains(t, stderr, "Continue: ")

	token := strings.TrimSpace(strings.TrimPrefix(stderr, "Continue: "))
	path, offset, err := cmdutil.DecodeContinueToken("teamcity agent list", token)
	require.NoError(t, err)
	assert.Equal(t, "/app/rest/agents?locator=count:2,start:2", path)
	assert.Zero(t, offset)
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
	cmdtest.RunCmdWithFactory(T, f, "agent", "reboot", "1", "--graceful")
	cmdtest.RunCmdWithFactory(T, f, "agent", "reboot", "1", "--after-build")
}
