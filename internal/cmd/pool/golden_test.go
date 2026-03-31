package pool_test

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func init() { color.NoColor = true }

func TestPoolList_plain(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "pool", "list", "--plain")
	want := "" +
		"ID\tNAME        \tMAX AGENTS\n" +
		"0 \tDefault     \tunlimited \n" +
		"1 \tLinux Agents\t10        \n"
	assert.Equal(t, want, got)
}

func TestPoolList_plain_no_header(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "pool", "list", "--plain", "--no-header")
	want := "" +
		"0 \tDefault     \tunlimited \n" +
		"1 \tLinux Agents\t10        \n"
	assert.Equal(t, want, got)
}

func TestPoolView_output(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	got := cmdtest.CaptureOutput(t, ts.Factory, "pool", "view", "0")
	want := cmdtest.Dedent(`
		Default
		ID: 0
		Max Agents: unlimited

		Agents (1)
		  1  Agent 1  Connected

		Projects (1)
		  _Root  Root project

		View in browser: ` + ts.URL + `/agents.html?tab=agentPools&poolId=0
	`)
	assert.Equal(t, want, got)
}
