//go:build !windows

package terminal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecInteractiveShell(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("requires sh")
	}
	for _, tt := range []struct {
		name, command string
		code          int
		incomplete    bool
	}{
		{name: "output without newline", command: "printf hello"},
		{name: "nonzero exit", command: "sh -c 'exit 7'", code: 7},
		{name: "old marker in output", command: "printf '__TC_EXEC_7f3a9e2b__:0\\n'; false", code: 1},
		{name: "shell exit has no completion trailer", command: "exit 0", incomplete: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer ws.Close()
				process := exec.CommandContext(t.Context(), shell, "-i")
				process.Env = append(os.Environ(), "ENV=", "PS1=ready> ", "TERM=dumb")
				terminal, err := pty.StartWithSize(process, &pty.Winsize{Rows: 24, Cols: 80})
				if err != nil {
					t.Errorf("start shell: %v", err)
					return
				}
				defer func() { _ = terminal.Close(); _ = process.Process.Kill(); _ = process.Wait() }()
				outputDone := make(chan struct{})
				go func() {
					defer close(outputDone)
					defer ws.Close()
					buf := make([]byte, 4096)
					for {
						n, err := terminal.Read(buf)
						if n > 0 {
							if ws.WriteMessage(websocket.TextMessage, buf[:n]) != nil {
								return
							}
						}
						if err != nil {
							return
						}
					}
				}()
				for {
					_, data, err := ws.ReadMessage()
					if err != nil {
						break
					}
					if _, err = terminal.Write(data); err != nil {
						break
					}
				}
				_ = terminal.Close()
				<-outputDone
			}))
			defer server.Close()
			ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			require.NoError(t, err)
			conn := &Conn{conn: ws, done: make(chan struct{})}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			err = conn.Exec(ctx, tt.command)
			switch {
			case tt.incomplete:
				require.ErrorContains(t, err, "before command completed")
			case tt.code != 0:
				var exit *cmdutil.ExitError
				require.ErrorAs(t, err, &exit)
				assert.Equal(t, tt.code, exit.Code)
			default:
				require.NoError(t, err)
			}
		})
	}
}
