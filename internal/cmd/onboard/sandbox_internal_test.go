package onboard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isolateAgentConfigs points Claude Code's and Codex's config dirs at empty temp dirs, so tests never read or write the real ones.
func isolateAgentConfigs(t *testing.T) (claudeDir, codexDir string) {
	t.Helper()
	claudeDir, codexDir = t.TempDir(), t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("CODEX_HOME", codexDir)
	return claudeDir, codexDir
}

// failOnConfirm fails the test if anything asks a question.
func failOnConfirm(t *testing.T) {
	t.Helper()
	orig := confirm
	confirm = func(title string, _ *bool) error {
		t.Errorf("unexpected question: %s", title)
		return nil
	}
	t.Cleanup(func() { confirm = orig })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestClaudeSandboxEnabled(t *testing.T) {
	claudeDir, _ := isolateAgentConfigs(t)
	repo := t.TempDir()
	assert.False(t, claudeSandboxEnabled(repo), "off by default")

	writeFile(t, filepath.Join(claudeDir, "settings.json"), `{"sandbox":{"enabled":true}}`)
	assert.True(t, claudeSandboxEnabled(repo), "user settings")

	writeFile(t, filepath.Join(repo, ".claude", "settings.json"), `{"sandbox":{"enabled":false}}`)
	assert.False(t, claudeSandboxEnabled(repo), "project overrides user")

	writeFile(t, filepath.Join(repo, ".claude", "settings.local.json"), `{"sandbox":{"enabled":true}}`)
	assert.True(t, claudeSandboxEnabled(repo), "local overrides project, as /sandbox saves it")
}

func TestCodexSandboxEnabled(t *testing.T) {
	_, codexDir := isolateAgentConfigs(t)
	assert.True(t, codexSandboxEnabled(), "on by default")

	writeFile(t, filepath.Join(codexDir, "config.toml"), `sandbox_mode = "workspace-write"`)
	assert.True(t, codexSandboxEnabled())

	writeFile(t, filepath.Join(codexDir, "config.toml"), `sandbox_mode = "danger-full-access"`)
	assert.False(t, codexSandboxEnabled())
}

func TestCodexTeamcityRule(t *testing.T) {
	_, codexDir := isolateAgentConfigs(t)
	assert.False(t, codexTeamcityRuleExists())

	require.NoError(t, writeCodexTeamcityRule())
	data, err := os.ReadFile(filepath.Join(codexDir, "rules", codexRuleFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), `prefix_rule(pattern=["teamcity"], decision="allow")`)
	assert.True(t, codexTeamcityRuleExists())
	assert.Error(t, writeCodexTeamcityRule(), "never overwrites the file")
}

func TestCodexTeamcityRuleExistsFromApproval(t *testing.T) {
	_, codexDir := isolateAgentConfigs(t)
	writeFile(t, filepath.Join(codexDir, "rules", "default.rules"), `prefix_rule(pattern=["teamcity"], decision="allow")`)
	assert.True(t, codexTeamcityRuleExists(), "a prefix the user approved for good")
}

func TestAllowOutsideSandbox(t *testing.T) {
	claude, codex, junie := supportedAgents[0], supportedAgents[1], supportedAgents[2]

	t.Run("claude not sandboxed asks nothing", func(t *testing.T) {
		isolateAgentConfigs(t)
		failOnConfirm(t)
		f, _ := testFactory()
		got, err := allowOutsideSandbox(f, claude, t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, api.OnboardingSandboxNotSandboxed, got.status)
		assert.False(t, got.agentMustAsk)
		assert.Empty(t, got.launchArgs)
	})

	t.Run("claude sandboxed, yes", func(t *testing.T) {
		claudeDir, _ := isolateAgentConfigs(t)
		writeFile(t, filepath.Join(claudeDir, "settings.json"), `{"sandbox":{"enabled":true}}`)
		stubConfirm(t, true)
		f, _ := testFactory()
		got, err := allowOutsideSandbox(f, claude, t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, api.OnboardingSandboxAllowed, got.status)
		assert.False(t, got.agentMustAsk)
		assert.Equal(t, []string{"--settings", `{"sandbox":{"excludedCommands":["teamcity *"]}}`}, got.launchArgs)
	})

	t.Run("claude sandboxed, no", func(t *testing.T) {
		claudeDir, _ := isolateAgentConfigs(t)
		writeFile(t, filepath.Join(claudeDir, "settings.json"), `{"sandbox":{"enabled":true}}`)
		stubConfirm(t, false)
		f, _ := testFactory()
		got, err := allowOutsideSandbox(f, claude, t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, api.OnboardingSandboxDeclined, got.status)
		assert.True(t, got.agentMustAsk)
		assert.Empty(t, got.launchArgs)
	})

	t.Run("codex yes writes the rule", func(t *testing.T) {
		_, codexDir := isolateAgentConfigs(t)
		stubConfirm(t, true)
		f, _ := testFactory()
		got, err := allowOutsideSandbox(f, codex, t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, api.OnboardingSandboxAllowed, got.status)
		assert.False(t, got.agentMustAsk)
		assert.Empty(t, got.launchArgs)
		assert.FileExists(t, filepath.Join(codexDir, "rules", codexRuleFile))
	})

	t.Run("codex no writes nothing", func(t *testing.T) {
		_, codexDir := isolateAgentConfigs(t)
		stubConfirm(t, false)
		f, _ := testFactory()
		got, err := allowOutsideSandbox(f, codex, t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, api.OnboardingSandboxDeclined, got.status)
		assert.True(t, got.agentMustAsk)
		assert.NoFileExists(t, filepath.Join(codexDir, "rules", codexRuleFile))
	})

	t.Run("junie asks nothing", func(t *testing.T) {
		isolateAgentConfigs(t)
		failOnConfirm(t)
		f, _ := testFactory()
		got, err := allowOutsideSandbox(f, junie, t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, api.OnboardingSandboxNotSandboxed, got.status)
		assert.False(t, got.agentMustAsk)
		assert.Empty(t, got.launchArgs)
	})
}
