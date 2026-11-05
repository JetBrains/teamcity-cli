//go:build !windows

package onboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnboardChecksAgentsBeforeRepository(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "teamcity")
	build := exec.Command("go", "build", "-o", binary, "github.com/JetBrains/teamcity-cli/tc")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "%s", out)

	terminal, stdin, err := pty.Open()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = terminal.Close()
		_ = stdin.Close()
	})
	client := newTestClient(t, map[string]http.HandlerFunc{
		"GET /app/rest/users/current": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, api.User{ID: 1, Username: "admin"})
		},
	})
	scratch := t.TempDir()
	configDir := filepath.Join(scratch, "tc")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	saved, err := json.Marshal(map[string]any{
		"default_server": client.ServerURL(),
		"servers":        map[string]any{client.ServerURL(): map[string]string{"token": "saved-token"}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yml"), saved, 0o600))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "onboard", client.ServerURL())
	command.Dir = scratch
	command.Env = append(os.Environ(), "XDG_CONFIG_HOME="+scratch, "PATH="+t.TempDir(),
		"TEAMCITY_URL=", "TEAMCITY_TOKEN=", "TEAMCITY_NO_UPDATE=1", "DO_NOT_TRACK=1", "NO_COLOR=1")
	var output bytes.Buffer
	command.Stdin, command.Stdout, command.Stderr = stdin, &output, &output

	err = command.Run()
	require.NoError(t, ctx.Err(), "onboarding waited for input: %s", output.String())
	require.NoError(t, err, "%s", output.String())
	assert.Contains(t, output.String(), "No supported agents found.")
	assert.Contains(t, output.String(), "Supported agents: Claude Code, Codex, Junie")
	assert.Contains(t, output.String(), "Install one, then rerun without --token: teamcity onboard "+client.ServerURL())
	assert.NotContains(t, output.String(), "Which repository")
}
