//go:build !windows

package auth_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebLoginDoesNotPromptForServerInTerminal(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "teamcity")
	build := exec.Command("go", "build", "-o", binary, "github.com/JetBrains/teamcity-cli/tc")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "%s", out)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	unreachable := "http://" + listener.Addr().String()
	require.NoError(t, listener.Close())
	for _, tc := range []struct {
		name, server, message string
	}{
		{"missing", "", "--server is required"},
		{"unreachable", unreachable, "cannot reach TeamCity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			terminal, stdin, err := pty.Open()
			require.NoError(t, err)
			t.Cleanup(func() { _ = terminal.Close(); _ = stdin.Close() })
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			args := []string{"auth", "login", "--web"}
			if tc.server != "" {
				args = append(args, "--server", tc.server)
			}
			command := exec.CommandContext(ctx, binary, args...)
			command.Dir = t.TempDir()
			command.Env = append(os.Environ(), "HOME="+t.TempDir(), "XDG_CONFIG_HOME="+t.TempDir(),
				"TEAMCITY_URL=", "TEAMCITY_TOKEN=", "NO_COLOR=1")
			var output bytes.Buffer
			command.Stdin, command.Stdout, command.Stderr = stdin, &output, &output
			err = command.Run()
			require.NoError(t, ctx.Err(), "login waited for terminal input: %s", output.String())
			require.Error(t, err)
			assert.Contains(t, output.String(), tc.message)
			assert.NotContains(t, output.String(), "TeamCity server URL")
		})
	}
}
