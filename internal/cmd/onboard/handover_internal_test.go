package onboard

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
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

	got, err := detectAgents(t.Context())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "claude-code", got[0].name)
	assert.Equal(t, claude, got[0].path)
	assert.Equal(t, "2.1.0 (Claude Code)", got[0].version)
}

func TestDetectAgentsCancelled(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	fakeBin(t, dir, "claude", "echo 2.1.0")
	t.Setenv("PATH", dir)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	got, err := detectAgents(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, got)
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

// stubConfirm answers every confirm prompt with answer for the test.
func stubConfirm(t *testing.T, answer bool) {
	t.Helper()
	orig := confirm
	confirm = func(_ string, v *bool) error {
		*v = answer
		return nil
	}
	t.Cleanup(func() { confirm = orig })
}

func TestHandOverToAgentNeedsLocalCopy(t *testing.T) {
	client, rec := newRecordingClient(t, nil) // any other request fails the test
	f, _ := testFactory()

	err := handOverToAgent(f, client, repository{source: sourceSample, remoteURL: "https://github.com/acme/app"}, "Instance_App", true)
	assert.ErrorContains(t, err, "needs a local copy")
	assert.Equal(t, []string{"handover-started", "handover-finished"}, rec.endpoints())
	assert.JSONEq(t, `{"repositoryType":"demo"}`, rec.body(t, "handover-started"))
	assert.JSONEq(t, `{"repositoryType":"demo","status":"no_checkout"}`, rec.body(t, "handover-finished"))
}

func TestHandOverToAgentNoSupportedAgents(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	client, rec := newRecordingClient(t, nil) // any other request fails the test
	f, out := testFactory()

	require.NoError(t, handOverToAgent(f, client, repository{source: sourceCurrent, dir: t.TempDir()}, "Instance_App", true))
	assert.Contains(t, out.String(), "No supported agents found, install one and try again.")
	assert.Contains(t, out.String(), "Supported agents: Claude Code, Codex, Junie")
	assert.Equal(t, []string{"handover-started", "agents-detected", "handover-finished"}, rec.endpoints())
	assert.JSONEq(t, `{"agents":["none"]}`, rec.body(t, "agents-detected"))
	assert.JSONEq(t, `{"repositoryType":"own","status":"no_agent"}`, rec.body(t, "handover-finished"))
}

func TestHandOverToAgentReportsDetectedAgent(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	fakeBin(t, dir, "claude", "echo 2.1.0")
	t.Setenv("PATH", dir)
	stubConfirm(t, false) // decline, so no skills get installed
	client, rec := newRecordingClient(t, nil)
	f, _ := testFactory()

	require.NoError(t, handOverToAgent(f, client, repository{source: sourcePath, dir: t.TempDir()}, "Instance_App", true))
	assert.JSONEq(t, `{"agents":["claude_code"]}`, rec.body(t, "agents-detected"))
	assert.JSONEq(t, `{"repositoryType":"own","status":"handover_declined"}`, rec.body(t, "handover-finished"))
}

func TestHandOverToAgentIgnoresOlderServer(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	run := func(status int) (string, error) {
		client, rec := newRecordingClient(t, nil)
		rec.respond(status)
		f, out := testFactory()
		err := handOverToAgent(f, client, repository{dir: t.TempDir()}, "Instance_App", true)
		assert.Len(t, rec.endpoints(), 3)
		return out.String(), err
	}

	wantOut, wantErr := run(http.StatusNoContent)
	gotOut, gotErr := run(http.StatusNotFound)
	assert.Equal(t, wantErr, gotErr)
	assert.Equal(t, wantOut, gotOut)
}

func TestHandOverToAgentSkippedWhenOptedOut(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	client, rec := newRecordingClient(t, nil)
	t.Setenv("DO_NOT_TRACK", "1")
	f, _ := testFactory()

	require.NoError(t, handOverToAgent(f, client, repository{dir: t.TempDir()}, "Instance_App", true))
	assert.Empty(t, rec.endpoints())
}

func TestHandoverStatus(t *testing.T) {
	tests := []struct {
		end  handoverEnd
		want api.OnboardingHandoverStatus
	}{
		{endNoCheckout, "no_checkout"},
		{endNoAgent, "no_agent"},
		{endDeclined, "handover_declined"},
		{endSkillInstallFailed, "skill_install_failure"},
		{endAgentStartFailed, "agent_failed_to_start"},
		{endAgentInterrupted, "agent_interrupted"},
		{endAgentError, "agent_error"},
		{endRerunDeclined, "handover_rerun_declined"},
		{endAgentExited, "success"},
		{endFailed, "error"},
	}
	for _, tc := range tests {
		t.Run(string(tc.want), func(t *testing.T) {
			assert.Equal(t, tc.want, handoverStatus(tc.end))
		})
	}
}

func TestRepositoryType(t *testing.T) {
	assert.Equal(t, api.OnboardingRepositoryDemo, repositoryType(sourceSample))
	assert.Equal(t, api.OnboardingRepositoryOwn, repositoryType(sourceCurrent))
	assert.Equal(t, api.OnboardingRepositoryOwn, repositoryType(sourcePath))
}

func TestSkillInstallCmd(t *testing.T) {
	claude := supportedAgents[0]

	t.Run("global", func(t *testing.T) {
		f, _ := testFactory()
		cmd := skillInstallCmd(f, "/bin/teamcity", claude, scopeGlobal, "/work/app")
		assert.Equal(t, []string{"/bin/teamcity", "skill", "install", "teamcity-cli", "teamcity-onboard", "--agent", "claude-code"}, cmd.Args)
	})

	t.Run("project installs into the target repo, not the current directory", func(t *testing.T) {
		f, _ := testFactory()
		cmd := skillInstallCmd(f, "/bin/teamcity", claude, scopeProject, "/work/app")
		assert.Equal(t, "--project", cmd.Args[len(cmd.Args)-1])
		assert.Equal(t, "/work/app", cmd.Dir)
	})

	t.Run("passes on quiet and no-color", func(t *testing.T) {
		f, _ := testFactory()
		f.Quiet, f.NoColor = true, true
		cmd := skillInstallCmd(f, "/bin/teamcity", claude, scopeGlobal, "/work/app")
		assert.Equal(t, []string{"--quiet", "--no-color"}, cmd.Args[len(cmd.Args)-2:])
	})
}
