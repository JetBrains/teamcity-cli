package skill_test

import (
	"os"
	"path/filepath"
	"testing"

	teamcitycli "github.com/JetBrains/teamcity-cli"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
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
		{"skill", "guide", "--help"},
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

	skillDir := filepath.Join(tmp, ".claude", "skills", "teamcity-cli")

	cmdtest.RunCmd(t, "skill", "install", "--agent", "claude-code")
	stub, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	require.NoError(t, err, "SKILL.md should exist after install")
	assert.Contains(t, string(stub), "teamcity skill guide teamcity-cli", "SKILL.md should be a stub pointing at the guide")

	_, err = os.Stat(filepath.Join(skillDir, "references"))
	assert.ErrorIs(t, err, os.ErrNotExist, "references are served by skill guide, not installed")

	cmdtest.RunCmd(t, "skill", "update", "--agent", "claude-code")

	cmdtest.RunCmd(t, "skill", "remove", "--agent", "claude-code")
	_, err = os.Stat(skillDir)
	assert.ErrorIs(t, err, os.ErrNotExist, "skill dir should be gone after remove")
}

// TestSkillEnumCoversBundledSkills fails when teamcity-skills adds a skill the FUS skill enum doesn't know.
func TestSkillEnumCoversBundledSkills(t *testing.T) {
	t.Parallel()
	for _, s := range teamcitycli.ListSkills() {
		assert.Equal(t, s.Name, analytics.NormalizeSkill(s.Name), "add %q to analytics.allSkills", s.Name)
	}
}

func TestSkillGuideUnknown(t *testing.T) {
	t.Parallel()
	cmdtest.RunCmdExpectErr(t, "unknown skill", "skill", "guide", "nonexistent-skill")
	cmdtest.RunCmdExpectErr(t, "unknown topic", "skill", "guide", "teamcity-cli", "nonexistent-topic")
}

func TestSkillInstallUnknownSkill(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(tmp, ".claude"))

	cmdtest.RunCmdExpectErr(t, "unknown skill", "skill", "install", "nonexistent-skill", "--agent", "claude-code")
}

func TestSkillInstallAllAndNameConflict(t *testing.T) {
	t.Parallel()
	cmdtest.RunCmdExpectErr(t, "cannot specify both", "skill", "install", "--all", "teamcity-cli", "--agent", "claude-code")
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
	_, err := os.Stat(filepath.Join(tmp, ".claude", "skills", "teamcity-cli", "SKILL.md"))
	require.NoError(t, err, "global install should write to CLAUDE_CONFIG_DIR")
	cmdtest.RunCmd(t, "skill", "remove", "--agent", "claude-code")
}
