//go:build !windows

package run_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunProgramStatusHelper runs commands in a separate process so Bubble Tea owns a real terminal.
func TestRunProgramStatusHelper(t *testing.T) {
	if os.Getenv("TC_STATUS_TEST_HELPER") != "1" {
		return
	}
	var args []string
	require.NoError(t, json.Unmarshal([]byte(os.Getenv("TC_STATUS_TEST_ARGS")), &args))
	f := cmdutil.NewFactory()
	f.SetContext(t.Context())
	f.ClientFunc = func() (api.ClientInterface, error) {
		return api.NewClient(os.Getenv("TC_STATUS_TEST_URL"), "test-token").WithContext(f.Context()), nil
	}
	command := cmd.NewCommand(f)
	command.SetArgs(args)
	// The parent asserts terminal outcomes; command failures are expected in several scenarios.
	if err := command.Execute(); err != nil {
		t.Logf("command returned: %v", err)
	}
}

func TestRunProgramStatusTerminal(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		args                  []string
		running, failed, quit bool
		want                  string
	}{
		{name: "watch success", args: []string{"run", "watch", "123"}, want: "done"},
		{name: "watch failure", args: []string{"run", "watch", "123"}, failed: true, want: "error"},
		{name: "watch timeout", args: []string{"run", "watch", "123", "--timeout", "50ms"}, running: true, want: "error"},
		{name: "watch JSON", args: []string{"run", "watch", "123", "--json"}},
		{name: "follow success", args: []string{"run", "log", "123", "--follow"}, want: "done"},
		{name: "follow failure", args: []string{"run", "log", "123", "--follow"}, failed: true, want: "error"},
		{name: "TUI completion", args: []string{"run", "watch", "123", "--logs"}, want: "done"},
		{name: "TUI early quit", args: []string{"run", "watch", "123", "--logs"}, running: true, quit: true, want: "idle"},
		{name: "download success", args: []string{"run", "download", "123"}, want: "done"},
		{name: "download failure", args: []string{"run", "download", "123"}, failed: true, want: "error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/artifacts/children"):
					_ = json.NewEncoder(w).Encode(api.Artifacts{File: []api.Artifact{{Name: "result.txt", Size: 2, Content: &api.Content{}}}})
				case strings.Contains(r.URL.Path, "/artifacts/content/"):
					if tt.failed {
						http.Error(w, "download failed", http.StatusNotFound)
						return
					}
					_, _ = io.WriteString(w, "ok")
				case r.URL.Path == "/app/rest/builds/id:123":
					build := api.Build{ID: 123, Number: "1", BuildTypeID: "Test", State: "finished", Status: "SUCCESS"}
					if tt.running {
						build.State = "running"
					}
					if tt.failed {
						build.Status = "FAILURE"
					}
					_ = json.NewEncoder(w).Encode(build)
				default:
					_, _ = io.WriteString(w, `{}`)
				}
			}))
			defer server.Close()
			args := tt.args
			if args[1] == "download" {
				args = append(args, "--output", t.TempDir())
			}
			encoded, err := json.Marshal(args)
			require.NoError(t, err)
			t.Setenv("TC_STATUS_TEST_HELPER", "1")
			t.Setenv("TC_STATUS_TEST_ARGS", string(encoded))
			t.Setenv("TC_STATUS_TEST_URL", server.URL)
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("TEAMCITY_URL", server.URL)
			t.Setenv("TEAMCITY_TOKEN", "test-token")
			t.Setenv("DO_NOT_TRACK", "1")
			t.Setenv("CI", "")
			t.Setenv("TEAMCITY_VERSION", "")
			t.Setenv("TERM", "xterm-256color")

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunProgramStatusHelper$")
			terminal, err := pty.StartWithSize(child, &pty.Winsize{Rows: 24, Cols: 100})
			require.NoError(t, err)
			defer terminal.Close()
			stopClose := context.AfterFunc(ctx, func() { _ = terminal.Close() })
			defer stopClose()
			var capture strings.Builder
			buffer := make([]byte, 4096)
			sentQuit := false
			answeredColor := false
			var quitTimer *time.Timer
			defer func() {
				if quitTimer != nil {
					quitTimer.Stop()
				}
			}()
			for {
				n, readErr := terminal.Read(buffer)
				capture.Write(buffer[:n])
				// Answer the UI library's startup query instead of waiting for its terminal-detection timeout.
				if !answeredColor && strings.Contains(capture.String(), "\x1b]11;?") {
					answeredColor = true
					_, _ = terminal.Write([]byte("\x1b]11;rgb:0000/0000/0000\x1b\\\x1b[1;1R"))
				}
				if tt.quit && !sentQuit && strings.Contains(capture.String(), "\x1b[?1049h") {
					sentQuit = true
					quitTimer = time.AfterFunc(100*time.Millisecond, func() { _, _ = terminal.Write([]byte("q")) })
				}
				if readErr != nil {
					break
				} // A closed PTY can return EIO rather than EOF.
			}
			err = child.Wait()
			require.NoError(t, ctx.Err(), "command hung: %q", capture.String())
			require.NoError(t, err, "helper failed: %q", capture.String())
			reports := capture.String()
			if tt.want == "" {
				assert.NotContains(t, reports, "7501;")
				return
			}
			assert.Regexp(t, `(?s)7501;state=working.*7501;state=`+tt.want, reports)
			if tt.quit {
				assert.True(t, sentQuit, "TUI never entered the alternate screen")
				assert.NotContains(t, reports, "7501;state=done")
			}
		})
	}
}
