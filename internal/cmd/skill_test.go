package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillHelp(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"skill", "--help"},
		{"skill", "install", "--help"},
		{"skill", "update", "--help"},
		{"skill", "remove", "--help"},
	} {
		t.Run(args[1], func(t *testing.T) {
			t.Parallel()
			runCmd(t, args...)
		})
	}
}

func TestSkillInstallRemove(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude"))

	skillDir := filepath.Join(tmp, ".claude", "skills", "teamcity-cli")

	runCmd(t, "skill", "install", "--agent", "claude-code")
	_, err := os.Stat(filepath.Join(skillDir, "SKILL.md"))
	require.NoError(t, err, "SKILL.md should exist after install")

	refs, err := os.ReadDir(filepath.Join(skillDir, "references"))
	require.NoError(t, err, "references dir should exist")
	assert.NotEmpty(t, refs, "references should contain files")

	runCmd(t, "skill", "update", "--agent", "claude-code")

	runCmd(t, "skill", "remove", "--agent", "claude-code")
	_, err = os.Stat(skillDir)
	assert.True(t, os.IsNotExist(err), "skill dir should be gone after remove")
}

func TestSkillRemoveNotInstalled(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude"))
	runCmd(t, "skill", "remove", "--agent", "claude-code")
}

func TestSkillInstallUnknownAgent(t *testing.T) {
	t.Parallel()
	runCmdExpectErr(t, "unknown agent", "skill", "install", "--agent", "bogus")
}

func TestSkillNoAgentsDetected(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude-none"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config-none"))
	t.Setenv("CODEX_HOME", filepath.Join(tmp, ".codex-none"))

	runCmdExpectErr(t, "no AI coding agents detected", "skill", "install")
}

func TestSkillProjectMode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude"))
	runCmd(t, "skill", "install", "--agent", "claude-code")
	_, err := os.Stat(filepath.Join(tmp, ".claude", "skills", "teamcity-cli", "SKILL.md"))
	require.NoError(t, err, "global install should write to CLAUDE_CONFIG_DIR")
	runCmd(t, "skill", "remove", "--agent", "claude-code")
}
