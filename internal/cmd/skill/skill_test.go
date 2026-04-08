package skill_test

import (
	"os"
	"path/filepath"
	"testing"

	teamcitycli "github.com/JetBrains/teamcity-cli"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillHelp(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"skill", "--help"},
		{"skill", "list", "--help"},
		{"skill", "install", "--help"},
		{"skill", "update", "--help"},
		{"skill", "remove", "--help"},
	} {
		t.Run(args[1], func(t *testing.T) {
			t.Parallel()
			cmdtest.RunCmd(t, args...)
		})
	}
}

func TestSkillList(t *testing.T) {
	t.Parallel()
	cmdtest.RunCmd(t, "skill", "list")
}

func TestSkillInstallRemoveDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude"))

	skillDir := filepath.Join(tmp, ".claude", "skills", teamcitycli.DefaultSkill)

	cmdtest.RunCmd(t, "skill", "install", "--agent", "claude-code")
	_, err := os.Stat(filepath.Join(skillDir, "SKILL.md"))
	require.NoError(t, err, "SKILL.md should exist after install")

	refs, err := os.ReadDir(filepath.Join(skillDir, "references"))
	require.NoError(t, err, "references dir should exist")
	assert.NotEmpty(t, refs, "references should contain files")

	cmdtest.RunCmd(t, "skill", "update", "--agent", "claude-code")

	cmdtest.RunCmd(t, "skill", "remove", "--agent", "claude-code")
	_, err = os.Stat(skillDir)
	assert.True(t, os.IsNotExist(err), "skill dir should be gone after remove")
}

func TestSkillInstallByName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude"))

	cmdtest.RunCmd(t, "skill", "install", teamcitycli.DefaultSkill, "--agent", "claude-code")
	skillDir := filepath.Join(tmp, ".claude", "skills", teamcitycli.DefaultSkill)
	_, err := os.Stat(filepath.Join(skillDir, "SKILL.md"))
	require.NoError(t, err, "SKILL.md should exist after named install")

	cmdtest.RunCmd(t, "skill", "remove", teamcitycli.DefaultSkill, "--agent", "claude-code")
}

func TestSkillInstallAll(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude"))

	cmdtest.RunCmd(t, "skill", "install", "--all", "--agent", "claude-code")

	skills := teamcitycli.ListSkills()
	for _, s := range skills {
		skillDir := filepath.Join(tmp, ".claude", "skills", s.Name)
		_, err := os.Stat(filepath.Join(skillDir, "SKILL.md"))
		require.NoError(t, err, "SKILL.md should exist for %s after --all install", s.Name)
	}

	cmdtest.RunCmd(t, "skill", "remove", "--all", "--agent", "claude-code")
}

func TestSkillInstallUnknownSkill(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude"))

	cmdtest.RunCmdExpectErr(t, "unknown skill", "skill", "install", "nonexistent-skill", "--agent", "claude-code")
}

func TestSkillInstallAllAndNameConflict(t *testing.T) {
	t.Parallel()
	cmdtest.RunCmdExpectErr(t, "cannot specify both", "skill", "install", "--all", teamcitycli.DefaultSkill, "--agent", "claude-code")
}

func TestSkillRemoveNotInstalled(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude"))
	cmdtest.RunCmd(t, "skill", "remove", "--agent", "claude-code")
}

func TestSkillInstallUnknownAgent(t *testing.T) {
	t.Parallel()
	cmdtest.RunCmdExpectErr(t, "unknown agent", "skill", "install", "--agent", "bogus")
}

func TestSkillNoAgentsDetected(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude-none"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config-none"))
	t.Setenv("CODEX_HOME", filepath.Join(tmp, ".codex-none"))

	cmdtest.RunCmdExpectErr(t, "no AI coding agents detected", "skill", "install")
}

func TestSkillProjectMode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude"))
	cmdtest.RunCmd(t, "skill", "install", "--agent", "claude-code")
	_, err := os.Stat(filepath.Join(tmp, ".claude", "skills", teamcitycli.DefaultSkill, "SKILL.md"))
	require.NoError(t, err, "global install should write to CLAUDE_CONFIG_DIR")
	cmdtest.RunCmd(t, "skill", "remove", "--agent", "claude-code")
}
