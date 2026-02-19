package cmd_test

import (
	"bytes"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAliasTest(t *testing.T) {
	t.Helper()
	t.Setenv("TEAMCITY_URL", "https://test.example.com")
	t.Setenv("TEAMCITY_TOKEN", "test-token")
	config.SetConfigPathForTest(t.TempDir() + "/config.yml")
	config.ResetForTest()
}

func TestAliasSetAndList(t *testing.T) {
	setupAliasTest(t)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "set", "fl", "run list --status=failure"})
	require.NoError(t, root.Execute())

	// Verify via JSON output (table output goes to os.Stdout, not capturable)
	var out bytes.Buffer
	root = cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "list", "--json"})
	root.SetOut(&out)
	require.NoError(t, root.Execute())
	assert.Contains(t, out.String(), "fl")
	assert.Contains(t, out.String(), "run list --status=failure")
}

func TestAliasSetShellFlag(t *testing.T) {
	setupAliasTest(t)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "set", "--shell", "failing", "teamcity run list | jq ."})
	require.NoError(t, root.Execute())

	exp, ok := config.GetAlias("failing")
	assert.True(t, ok)
	assert.Equal(t, "!teamcity run list | jq .", exp)
}

func TestAliasSetBangPrefix(t *testing.T) {
	setupAliasTest(t)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "set", "failing", "!tc run list | jq ."})
	require.NoError(t, root.Execute())

	assert.True(t, config.IsShellAlias("failing"))
}

func TestAliasSetRejectsBuiltin(t *testing.T) {
	setupAliasTest(t)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "set", "run", "job list"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "built-in command")
}

func TestAliasSetOverwrite(t *testing.T) {
	setupAliasTest(t)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "set", "fl", "run list --status=failure"})
	require.NoError(t, root.Execute())

	root = cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "set", "fl", "run list --status=success"})
	require.NoError(t, root.Execute())

	exp, _ := config.GetAlias("fl")
	assert.Equal(t, "run list --status=success", exp)
}

func TestAliasDeleteCmd(t *testing.T) {
	setupAliasTest(t)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "set", "fl", "run list"})
	require.NoError(t, root.Execute())

	root = cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "delete", "fl"})
	require.NoError(t, root.Execute())

	_, ok := config.GetAlias("fl")
	assert.False(t, ok)
}

func TestAliasDeleteNonexistentCmd(t *testing.T) {
	setupAliasTest(t)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "delete", "nope"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no such alias")
}

func TestAliasListEmpty(t *testing.T) {
	setupAliasTest(t)

	var out bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "list"})
	root.SetOut(&out)
	require.NoError(t, root.Execute())
	assert.Contains(t, out.String(), "No aliases configured")
}

func TestAliasListJSON(t *testing.T) {
	setupAliasTest(t)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "set", "fl", "run list"})
	require.NoError(t, root.Execute())

	var out bytes.Buffer
	root = cmd.NewRootCmd()
	root.SetArgs([]string{"alias", "list", "--json"})
	root.SetOut(&out)
	require.NoError(t, root.Execute())
	assert.Contains(t, out.String(), `"name"`)
	assert.Contains(t, out.String(), `"fl"`)
}

// End-to-end tests: alias expansion through real command execution

func setupAliasE2E(t *testing.T) {
	t.Helper()
	_ = setupMockClient(t)
	config.SetConfigPathForTest(t.TempDir() + "/config.yml")
}

func TestAliasExpansionEndToEnd(t *testing.T) {
	setupAliasE2E(t)
	require.NoError(t, config.AddAlias("fl", "run list --status=failure"))

	root := cmd.NewRootCmd()
	cmd.RegisterAliases(root)

	var out bytes.Buffer
	root.SetArgs([]string{"fl"})
	root.SetOut(&out)
	root.SetErr(&out)
	require.NoError(t, root.Execute())
}

func TestAliasExpansionWithExtraArgs(t *testing.T) {
	setupAliasE2E(t)
	require.NoError(t, config.AddAlias("fl", "run list --status=failure"))

	root := cmd.NewRootCmd()
	cmd.RegisterAliases(root)

	var out bytes.Buffer
	root.SetArgs([]string{"fl", "--limit", "5"})
	root.SetOut(&out)
	root.SetErr(&out)
	require.NoError(t, root.Execute())
}

func TestAliasExpansionWithPositionalArgs(t *testing.T) {
	setupAliasE2E(t)
	require.NoError(t, config.AddAlias("mybuilds", "run list --user=$1 --status=success"))

	root := cmd.NewRootCmd()
	cmd.RegisterAliases(root)

	var out bytes.Buffer
	root.SetArgs([]string{"mybuilds", "admin"})
	root.SetOut(&out)
	root.SetErr(&out)
	require.NoError(t, root.Execute())
}

func TestAliasHelpFlag(t *testing.T) {
	setupAliasTest(t)
	require.NoError(t, config.AddAlias("fl", "run list --status=failure"))

	root := cmd.NewRootCmd()
	cmd.RegisterAliases(root)

	var out bytes.Buffer
	root.SetArgs([]string{"fl", "--help"})
	root.SetOut(&out)
	root.SetErr(&out)
	_ = root.Execute()
	assert.Contains(t, out.String(), "run list --status=failure")
}

func TestAliasDepthLimit(t *testing.T) {
	setupAliasTest(t)
	require.NoError(t, config.AddAlias("loop", "loop"))

	root := cmd.NewRootCmd()
	cmd.RegisterAliases(root)

	root.SetArgs([]string{"loop"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "depth limit exceeded")
}

func TestAwesomeAliasesEndToEnd(t *testing.T) {
	setupAliasE2E(t)

	aliases := map[string]string{
		"rl":      "run list",
		"mine":    "run list --user=@me",
		"fails":   "run list --status=failure --since=24h",
		"running": "run list --status=running",
		"morning": "run list --status=failure --since=12h",
		"whoami":  "api /app/rest/users/current",
	}
	for name, exp := range aliases {
		require.NoError(t, config.AddAlias(name, exp))
	}

	for name := range aliases {
		t.Run(name, func(t *testing.T) {
			root := cmd.NewRootCmd()
			cmd.RegisterAliases(root)

			var out bytes.Buffer
			root.SetArgs([]string{name})
			root.SetOut(&out)
			root.SetErr(&out)
			require.NoError(t, root.Execute())
		})
	}
}

func TestAliasSkipsBuiltinConflict(t *testing.T) {
	setupAliasTest(t)
	require.NoError(t, config.AddAlias("run", "job list"))

	root := cmd.NewRootCmd()
	cmd.RegisterAliases(root)

	var out bytes.Buffer
	root.SetArgs([]string{"run"})
	root.SetOut(&out)
	root.SetErr(&out)
	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a subcommand")
}
