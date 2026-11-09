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

	err := handOverToAgent(f, client, repository{source: sourceSample, remoteURL: "https://github.com/acme/app"}, "Instance_App", true, supportedAgents[:1])
	assert.ErrorContains(t, err, "needs a local copy")
	assert.Equal(t, []string{"handover-finished"}, rec.endpoints(), "no agent launched, so no handover-started")
	assert.JSONEq(t, `{"repositoryType":"demo","status":"no_checkout"}`, rec.body(t, "handover-finished"))
}

func TestHandOverToAgentDeclined(t *testing.T) {
	stubConfirm(t, false) // decline, so no skills get installed
	client, rec := newRecordingClient(t, nil)
	f, _ := testFactory()

	require.NoError(t, handOverToAgent(f, client, repository{source: sourcePath, dir: t.TempDir()}, "Instance_App", true, supportedAgents[:1]))
	assert.Equal(t, []string{"handover-finished"}, rec.endpoints(), "declined before the agent was launched")
	assert.JSONEq(t, `{"repositoryType":"own","status":"handover_declined"}`, rec.body(t, "handover-finished"))
}

func TestHandOverToAgentIgnoresOlderServer(t *testing.T) {
	stubConfirm(t, false)
	run := func(status int) (string, error) {
		client, rec := newRecordingClient(t, nil)
		rec.respond(status)
		f, out := testFactory()
		err := handOverToAgent(f, client, repository{dir: t.TempDir()}, "Instance_App", true, supportedAgents[:1])
		assert.Len(t, rec.endpoints(), 1)
		return out.String(), err
	}

	wantOut, wantErr := run(http.StatusNoContent)
	gotOut, gotErr := run(http.StatusNotFound)
	assert.Equal(t, wantErr, gotErr)
	assert.Equal(t, wantOut, gotOut)
}

func TestHandOverToAgentSkippedWhenOptedOut(t *testing.T) {
	stubConfirm(t, false)
	client, rec := newRecordingClient(t, nil)
	t.Setenv("DO_NOT_TRACK", "1")
	f, _ := testFactory()

	require.NoError(t, handOverToAgent(f, client, repository{dir: t.TempDir()}, "Instance_App", true, supportedAgents[:1]))
	assert.Empty(t, rec.endpoints())
}

func TestDetectAgentsNone(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	agents, err := detectAgents(t.Context())
	require.NoError(t, err)
	assert.Empty(t, agents)
}

func TestNotifyAgentsDetected(t *testing.T) {
	tests := []struct {
		name   string
		agents []agent
		want   string
	}{
		{"none", nil, `{"agents":["none"]}`},
		{"one", supportedAgents[:1], `{"agents":["claude_code"]}`},
		{"all", supportedAgents, `{"agents":["claude_code","codex","junie"]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, rec := newRecordingClient(t, nil)
			f, _ := testFactory()

			notifyAgentsDetected(f, client, tc.agents)
			assert.JSONEq(t, tc.want, rec.body(t, "agents-detected"))
		})
	}
}

func TestNotifyHandoverStarted(t *testing.T) {
	client, rec := newRecordingClient(t, nil)
	f, _ := testFactory()

	notifyHandoverStarted(f, client, api.OnboardingRepositoryDemo, supportedAgents[0])
	assert.JSONEq(t, `{"repositoryType":"demo","agent":"claude_code"}`, rec.body(t, "handover-started"))
}

func TestHandoverStatus(t *testing.T) {
	tests := []struct {
		end  handoverEnd
		want api.OnboardingHandoverStatus
	}{
		{endNoCheckout, "no_checkout"},
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
