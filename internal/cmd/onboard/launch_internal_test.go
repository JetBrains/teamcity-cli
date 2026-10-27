package onboard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAgent(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()

	t.Run("clean exit", func(t *testing.T) {
		assert.NoError(t, runAgent(fakeBin(t, dir, "ok", "exit 0"), nil, ""))
	})

	t.Run("failure keeps the exit code", func(t *testing.T) {
		err := runAgent(fakeBin(t, dir, "fail", "exit 3"), nil, "")
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		require.True(t, ok, "want *exec.ExitError, got %v", err)
		assert.Equal(t, 3, exitErr.ExitCode())
	})

	t.Run("missing executable is a start error", func(t *testing.T) {
		err := runAgent(filepath.Join(dir, "missing"), nil, "")
		require.Error(t, err)
		_, ok := errors.AsType[*exec.ExitError](err)
		assert.False(t, ok)
	})
}

func TestLaunchAgentRunsInRepoWithPrompt(t *testing.T) {
	skipOnWindows(t)
	bin, repoDir := t.TempDir(), t.TempDir()
	record := filepath.Join(t.TempDir(), "record")
	ag := agent{label: "Junie", bin: "junie", promptFlag: "--prompt"}
	ag.path = fakeBin(t, bin, "junie", `pwd -P > "`+record+`"; printf '%s\n' "$1" >> "`+record+`"`)
	f, out := testFactory()

	err := launchAgent(f, ag, handoverContext{serverURL: "https://tc.example.com", projectID: "Instance_App", repo: repository{dir: repoDir}})
	require.NoError(t, err)

	got, err := os.ReadFile(record)
	require.NoError(t, err)
	wantDir, err := filepath.EvalSymlinks(repoDir)
	require.NoError(t, err)
	assert.Equal(t, wantDir+"\n--prompt\n", string(got), "runs in the repo dir with the prompt flag first")
	assert.Contains(t, out.String(), "Junie exited.")
}

func TestLaunchAgentStartFailure(t *testing.T) {
	f, _ := testFactory()
	ag := agent{label: "Codex", bin: "codex", path: filepath.Join(t.TempDir(), "missing")}

	err := launchAgent(f, ag, handoverContext{})
	assert.ErrorContains(t, err, "start Codex")
}

func TestPromptArgs(t *testing.T) {
	assert.Equal(t, []string{"task"}, agent{}.promptArgs("task"))
	assert.Equal(t, []string{"--prompt", "task"}, agent{promptFlag: "--prompt"}.promptArgs("task"))
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

	t.Run("stays on one line for Windows .cmd shims", func(t *testing.T) {
		got := handoverPrompt(handoverContext{serverURL: "https://tc.example.com", projectID: "Instance_App", repo: repository{dir: "/work/app", remoteURL: "https://github.com/acme/app", revision: "abc123"}})
		assert.NotContains(t, got, "\n")
	})
}
