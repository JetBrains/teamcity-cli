package onboard

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAgent(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()

	t.Run("clean exit", func(t *testing.T) {
		assert.NoError(t, runAgent(fakeBin(t, dir, "ok", "exit 0"), nil, "", api.OnboardingRepositoryOwn))
	})

	t.Run("failure keeps the exit code", func(t *testing.T) {
		err := runAgent(fakeBin(t, dir, "fail", "exit 3"), nil, "", api.OnboardingRepositoryOwn)
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		require.True(t, ok, "want *exec.ExitError, got %v", err)
		assert.Equal(t, 3, exitErr.ExitCode())
	})

	t.Run("missing executable is a start error", func(t *testing.T) {
		err := runAgent(filepath.Join(dir, "missing"), nil, "", api.OnboardingRepositoryOwn)
		require.Error(t, err)
		_, ok := errors.AsType[*exec.ExitError](err)
		assert.False(t, ok)
	})
}

func TestLaunchAgentRunsInRepoWithPrompt(t *testing.T) {
	skipOnWindows(t)
	for source, want := range map[repoSource]string{sourceCurrent: "own", sourcePath: "own", sourceSample: "demo"} {
		t.Run(string(source), func(t *testing.T) {
			t.Setenv("TEAMCITY_ONBOARDING", "") // the host's value must not leak through
			bin, repoDir := t.TempDir(), t.TempDir()
			record := filepath.Join(t.TempDir(), "record")
			ag := agent{label: "Junie", bin: "junie", promptFlag: "--prompt"}
			ag.path = fakeBin(t, bin, "junie", `pwd -P > "`+record+`"; printf '%s\n' "$1" "$TEAMCITY_ONBOARDING" >> "`+record+`"`)
			f, out := testFactory()

			end, err := launchAgent(f, ag, handoverContext{serverURL: "https://tc.example.com", projectID: "Instance_App", repo: repository{source: source, dir: repoDir}})
			require.NoError(t, err)
			assert.Equal(t, api.OnboardingStatusSuccess, handoverStatus(end))

			got, err := os.ReadFile(record)
			require.NoError(t, err)
			wantDir, err := filepath.EvalSymlinks(repoDir)
			require.NoError(t, err)
			assert.Equal(t, wantDir+"\n--prompt\n"+want+"\n", string(got), "runs in the repo dir with the prompt flag first and the onboarding context")
			assert.Contains(t, out.String(), "Junie exited.")
		})
	}
}

func TestLaunchAgentCancelledSkipsRerun(t *testing.T) {
	skipOnWindows(t)
	ag := agent{label: "Codex", bin: "codex", path: fakeBin(t, t.TempDir(), "codex", "exit 130")}
	f, out := testFactory()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	f.SetContext(ctx)

	end, err := launchAgent(f, ag, handoverContext{repo: repository{dir: t.TempDir()}})
	exitErr, ok := errors.AsType[*cmdutil.ExitError](err)
	require.True(t, ok, "want *cmdutil.ExitError, got %v", err)
	assert.Equal(t, 130, exitErr.Code)
	assert.NotContains(t, out.String(), "stopped with an error")
	assert.Equal(t, api.OnboardingStatusAgentInterrupted, handoverStatus(end))
}

func TestLaunchAgentFailure(t *testing.T) {
	skipOnWindows(t)

	t.Run("rerun declined", func(t *testing.T) {
		stubConfirm(t, false)
		ag := agent{label: "Codex", bin: "codex", path: fakeBin(t, t.TempDir(), "codex", "exit 3")}
		f, out := testFactory()

		end, err := launchAgent(f, ag, handoverContext{repo: repository{dir: t.TempDir()}})
		exitErr, ok := errors.AsType[*cmdutil.ExitError](err)
		require.True(t, ok, "want *cmdutil.ExitError, got %v", err)
		assert.Equal(t, 3, exitErr.Code)
		assert.Contains(t, out.String(), "stopped with an error")
		assert.Equal(t, api.OnboardingStatusHandoverRerunDeclined, handoverStatus(end))
	})

	t.Run("agent gone on rerun", func(t *testing.T) {
		stubConfirm(t, true)
		t.Setenv("PATH", t.TempDir())
		ag := agent{label: "Codex", bin: "codex", path: fakeBin(t, t.TempDir(), "codex", "exit 3")}
		f, _ := testFactory()

		end, err := launchAgent(f, ag, handoverContext{repo: repository{dir: t.TempDir()}})
		assert.ErrorContains(t, err, "no longer on PATH")
		assert.Equal(t, api.OnboardingStatusAgentError, handoverStatus(end))
	})

	t.Run("rerun that succeeds", func(t *testing.T) {
		stubConfirm(t, true)
		bin := t.TempDir()
		fakeBin(t, bin, "codex", "exit 0") // the repaired install found on PATH
		t.Setenv("PATH", bin)
		ag := agent{label: "Codex", bin: "codex", path: fakeBin(t, t.TempDir(), "codex", "exit 3")}
		f, _ := testFactory()

		end, err := launchAgent(f, ag, handoverContext{repo: repository{dir: t.TempDir()}})
		require.NoError(t, err)
		assert.Equal(t, api.OnboardingStatusSuccess, handoverStatus(end), "an accepted rerun isn't final")
	})
}

func TestLaunchAgentStartFailure(t *testing.T) {
	f, _ := testFactory()
	ag := agent{label: "Codex", bin: "codex", path: filepath.Join(t.TempDir(), "missing")}

	end, err := launchAgent(f, ag, handoverContext{})
	assert.ErrorContains(t, err, "start Codex")
	assert.Equal(t, api.OnboardingStatusAgentFailedToStart, handoverStatus(end))
}

func TestPromptArgs(t *testing.T) {
	assert.Equal(t, []string{"task"}, agent{}.promptArgs("task", nil))
	assert.Equal(t, []string{"--prompt", "task"}, agent{promptFlag: "--prompt"}.promptArgs("task", nil))
	assert.Equal(t, []string{"--settings", "{}", "--prompt", "task"}, agent{promptFlag: "--prompt"}.promptArgs("task", []string{"--settings", "{}"}))
}

func TestWithoutUserinfo(t *testing.T) {
	for in, want := range map[string]string{
		"https://user:token@github.com/acme/app.git":    "https://github.com/acme/app.git",
		"https://ghp_token@github.com/acme/app.git":     "https://github.com/acme/app.git",
		"http://user:token@git.example.com/app":         "http://git.example.com/app",
		"https://user:apppw@bitbucket.org/acme/app.git": "https://bitbucket.org/acme/app.git",
		"https://github.com/acme/app.git":               "https://github.com/acme/app.git",
		"git@github.com:acme/app.git":                   "git@github.com:acme/app.git",
		"ssh://git@github.com/acme/app.git":             "ssh://git@github.com/acme/app.git",
	} {
		assert.Equal(t, want, withoutUserinfo(in), in)
	}
}

func TestHandoverPrompt(t *testing.T) {
	t.Run("authorized sample with revision", func(t *testing.T) {
		got := handoverPrompt(handoverContext{
			serverURL:  "https://tc.example.com",
			projectID:  "Instance_App",
			repo:       repository{dir: "/work/app", remoteURL: "https://github.com/acme/app", revision: "abc123"},
			authorized: true,
		})
		assert.Equal(t, "Use the teamcity-onboard skill to set up TeamCity for this repository. Server: https://tc.example.com. Project: Instance_App. "+
			"Repository: https://github.com/acme/app. Local checkout: /work/app. Revision: abc123. VCS root: exists.", got)
	})

	t.Run("no remote omits what wasn't staged", func(t *testing.T) {
		got := handoverPrompt(handoverContext{projectID: "Instance_App", repo: repository{dir: "/work/app"}})
		assert.NotContains(t, got, "Repository:")
		assert.NotContains(t, got, "Revision:")
		assert.Contains(t, got, "VCS root: none.")
	})

	t.Run("asks to leave the sandbox only when teamcity stays in it", func(t *testing.T) {
		assert.NotContains(t, handoverPrompt(handoverContext{projectID: "Instance_App"}), "sandbox")
		assert.Contains(t, handoverPrompt(handoverContext{projectID: "Instance_App", sandbox: sandboxOutcome{agentMustAsk: true}}), "Ask to run teamcity commands outside the sandbox")
	})

	t.Run("keeps remote credentials out of the command line", func(t *testing.T) {
		got := handoverPrompt(handoverContext{projectID: "Instance_App", repo: repository{dir: "/work/app", remoteURL: "https://user:secret@github.com/acme/app.git"}})
		assert.Contains(t, got, "Repository: https://github.com/acme/app.git.")
		assert.NotContains(t, got, "secret")
	})

	t.Run("stays on one line for Windows .cmd shims", func(t *testing.T) {
		got := handoverPrompt(handoverContext{serverURL: "https://tc.example.com", projectID: "Instance_App", repo: repository{dir: "/work/app", remoteURL: "https://github.com/acme/app", revision: "abc123"}})
		assert.NotContains(t, got, "\n")
	})
}
