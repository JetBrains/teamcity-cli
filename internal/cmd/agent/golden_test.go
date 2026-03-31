package agent_test

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func init() { color.NoColor = true }

func TestAgentList_plain(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "agent", "list", "--plain")
	// NOTE: trailing spaces on non-widest rows are from FillRight padding
	want := "ID\tNAME   \tPOOL   \tSTATUS      \n" +
		"1 \tAgent 1\tDefault\tConnected   \n" +
		"2 \tAgent 2\tDefault\tDisconnected\n"
	assert.Equal(t, want, got)
}

func TestAgentList_plain_no_header(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "agent", "list", "--plain", "--no-header")
	want := "1 \tAgent 1\tDefault\tConnected   \n" +
		"2 \tAgent 2\tDefault\tDisconnected\n"
	assert.Equal(t, want, got)
}

func TestAgentView_output(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "agent", "view", "1")
	want := cmdtest.Dedent(`
		Agent 1
		ID: 1
		Pool: Default
		Status: Connected
		Connected: Yes
		Enabled: Yes
		Authorized: Yes

		View in browser: ` + ts.URL + `/agentDetails.html?id=1
		Open terminal: teamcity agent term 1
	`)
	assert.Equal(t, want, got)
}
