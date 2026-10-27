package onboard

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tiulpin/instill"
)

// fakeBin writes an executable shell script named name into dir.
func fakeBin(t *testing.T, dir, name, script string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755))
	return path
}

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are shell scripts")
	}
}

func TestSupportedAgentsAreValid(t *testing.T) {
	for _, a := range supportedAgents {
		assert.Contains(t, instill.AgentNames(), a.name, "skill install must accept agent %q", a.name)
		assert.NotEmpty(t, a.label)
		assert.NotEmpty(t, a.bin)
	}
}

func TestSupportedAgentLabels(t *testing.T) {
	assert.Equal(t, "Claude Code, Codex, Junie", supportedAgentLabels())
}

func TestDetectAgents(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	claude := fakeBin(t, dir, "claude", `printf '2.1.0 (Claude Code)\nextra line\n'`)
	fakeBin(t, dir, "codex", "exit 1")            // broken install
	fakeBin(t, dir, "cursor-agent", "echo 1.0.0") // unsupported
	t.Setenv("PATH", dir)                         // junie is absent

	got := detectAgents(t.Context())
	require.Len(t, got, 1)
	assert.Equal(t, "claude-code", got[0].name)
	assert.Equal(t, claude, got[0].path)
	assert.Equal(t, "2.1.0 (Claude Code)", got[0].version)
}

func TestAgentVersionTimesOut(t *testing.T) {
	skipOnWindows(t)
	// A child process keeps stdout open after the shell is killed; WaitDelay must not wait for it.
	hang := fakeBin(t, t.TempDir(), "hang", "/bin/sleep 10")
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := agentVersion(ctx, hang)
	assert.Error(t, err)
	assert.Less(t, time.Since(start), 3*time.Second)
}

func TestPickAgentSingleSkipsPrompt(t *testing.T) {
	f, _ := testFactory()
	got, err := pickAgent(f, supportedAgents[:1])
	require.NoError(t, err)
	assert.Equal(t, supportedAgents[0], got)
}

func TestPromptSkillScopeWithoutCloneIsGlobal(t *testing.T) {
	f, _ := testFactory()
	got, err := promptSkillScope(f, repository{source: sourceSample, remoteURL: "https://github.com/acme/app"})
	require.NoError(t, err)
	assert.Equal(t, scopeGlobal, got)
}

func TestHandOverToAgentNoSupportedAgents(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	client := newTestClient(t, nil) // any request fails the test
	f, out := testFactory()

	require.NoError(t, handOverToAgent(f, client, repository{dir: t.TempDir()}, "Instance_App", true))
	assert.Contains(t, out.String(), "No supported agents found, install one and try again.")
	assert.Contains(t, out.String(), "Supported agents: Claude Code, Codex, Junie")
}

func TestInstallSkills(t *testing.T) {
	claude := supportedAgents[0]

	t.Run("global", func(t *testing.T) {
		configDir := filepath.Join(t.TempDir(), ".claude")
		t.Setenv("CLAUDE_CONFIG_DIR", configDir)
		f, _ := testFactory()

		require.NoError(t, installSkills(f, claude, scopeGlobal))
		assert.FileExists(t, filepath.Join(configDir, "skills", "teamcity-cli", "SKILL.md"))
	})

	t.Run("project installs into the current directory", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(t.TempDir(), ".claude"))
		project := t.TempDir()
		t.Chdir(project)
		f, _ := testFactory()

		require.NoError(t, installSkills(f, claude, scopeProject))
		assert.FileExists(t, filepath.Join(project, ".claude", "skills", "teamcity-cli", "SKILL.md"))
	})
}
