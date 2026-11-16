package onboard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/charmbracelet/huh"
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
		assert.Contains(t, analytics.OnboardingAgents(), analytics.NormalizeAIAgent(a.name), "the onboarding analytics must accept agent %q", a.name)
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

// stubConfirmErr makes confirm fail with err, like a prompt the user aborted.
func stubConfirmErr(t *testing.T, err error) {
	t.Helper()
	orig := confirm
	confirm = func(string, *bool) error { return err }
	t.Cleanup(func() { confirm = orig })
}

func TestHandOverToAgentNeedsLocalCopy(t *testing.T) {
	client, rec := newRecordingClient(t, nil) // any other request fails the test
	tracked := stubTrack(t)
	f, _ := testFactory()

	err := handOverToAgent(f, client, repository{source: sourceSample, remoteURL: "https://github.com/acme/app"}, "Instance_App", true, supportedAgents[:1])
	assert.ErrorContains(t, err, "needs a local copy")
	assert.Empty(t, rec.endpoints(), "the hand-over isn't reported to the server")
	assert.Equal(t, []trackedEvent{
		{analytics.EventHandoverFinished, map[string]any{"repository": "demo", "outcome": "no_checkout"}},
	}, *tracked)
}

func TestHandOverToAgentDeclined(t *testing.T) {
	stubConfirm(t, false) // decline, so no skills get installed
	client, rec := newRecordingClient(t, nil)
	tracked := stubTrack(t)
	f, _ := testFactory()

	require.NoError(t, handOverToAgent(f, client, repository{source: sourcePath, dir: t.TempDir()}, "Instance_App", true, supportedAgents[:1]))
	assert.Empty(t, rec.endpoints(), "the hand-over isn't reported to the server")
	assert.Equal(t, []trackedEvent{
		{analytics.EventHandoverFinished, map[string]any{"repository": "own", "outcome": "handover_declined"}},
	}, *tracked)
}

func TestHandOverToAgentPromptAborted(t *testing.T) {
	stubConfirmErr(t, huh.ErrUserAborted)
	client, _ := newRecordingClient(t, nil)
	tracked := stubTrack(t)
	f, _ := testFactory()

	err := handOverToAgent(f, client, repository{source: sourcePath, dir: t.TempDir()}, "Instance_App", true, supportedAgents[:1])
	assert.ErrorIs(t, err, huh.ErrUserAborted)
	assert.Equal(t, []trackedEvent{
		{analytics.EventHandoverFinished, map[string]any{"repository": "own", "outcome": "handover_declined"}},
	}, *tracked)
}

func TestPromptOutcome(t *testing.T) {
	declined := analytics.OnboardingOutcomeHandoverDeclined
	assert.Equal(t, declined, promptOutcome(huh.ErrUserAborted, declined))
	assert.Equal(t, analytics.OnboardingOutcomeError, promptOutcome(errors.New("no tty"), declined))
}

func TestTrackSandboxChecked(t *testing.T) {
	tracked := stubTrack(t)
	f, _ := testFactory()

	trackSandboxChecked(f, supportedAgents[1], analytics.OnboardingSandboxDeclined)
	assert.Equal(t, []trackedEvent{
		{analytics.EventSandboxChecked, map[string]any{"agent": "codex", "status": "declined"}},
	}, *tracked)
}

func TestTrackHandoverStarted(t *testing.T) {
	tracked := stubTrack(t)
	f, _ := testFactory()

	trackHandoverStarted(f, api.OnboardingRepositoryDemo, supportedAgents[0])
	assert.Equal(t, []trackedEvent{
		{analytics.EventHandoverStarted, map[string]any{"repository": "demo", "agent": "claude_code"}},
	}, *tracked)
}

func TestDetectAgentsNone(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	agents, err := detectAgents(t.Context())
	require.NoError(t, err)
	assert.Empty(t, agents)
}

func TestDetectedAgentFields(t *testing.T) {
	claude, codex, junie := supportedAgents[0], supportedAgents[1], supportedAgents[2]
	tests := []struct {
		name   string
		agents []agent
		want   map[string]any
	}{
		{"none", nil, map[string]any{"has_claude_code": false, "has_codex": false, "has_junie": false}},
		{"claude code", []agent{claude}, map[string]any{"has_claude_code": true, "has_codex": false, "has_junie": false}},
		{"codex", []agent{codex}, map[string]any{"has_claude_code": false, "has_codex": true, "has_junie": false}},
		{"junie", []agent{junie}, map[string]any{"has_claude_code": false, "has_codex": false, "has_junie": true}},
		{"several", []agent{claude, junie}, map[string]any{"has_claude_code": true, "has_codex": false, "has_junie": true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, detectedAgentFields(tc.agents))
		})
	}
}

// TestDetectedAgentFieldsMatchScheme fails when a new supported agent would send a has_<agent> field the scheme doesn't declare.
func TestDetectedAgentFieldsMatchScheme(t *testing.T) {
	var declared []string
	for _, g := range analytics.Definition.Groups {
		for _, e := range g.Events {
			if g.ID == analytics.GroupOnboarding && e.ID == analytics.EventAgentsDetected {
				for _, fd := range e.Fields {
					declared = append(declared, fd.Path)
				}
			}
		}
	}
	var sent []string
	for field := range detectedAgentFields(supportedAgents) {
		sent = append(sent, field)
	}
	assert.ElementsMatch(t, declared, sent)
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
