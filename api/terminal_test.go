//go:build integration

package api_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTerminalClient() *terminal.Client {
	user, pass := os.Getenv("TEAMCITY_USER"), os.Getenv("TEAMCITY_PASSWORD")
	if user == "" {
		user, pass = "admin", "admin123"
	}
	noop := func(string, ...any) {}
	return terminal.NewClient(client.BaseURL, user, pass, noop)
}

// openTerminalConn opens a terminal session and WebSocket connection with retries.
// After pool/agent state changes, the terminal endpoint may briefly return errors.
func openTerminalConn(t *testing.T, agentID int) *terminal.Conn {
	t.Helper()
	var lastErr error
	for range 5 {
		tc := getTerminalClient()
		session, err := tc.OpenSession(agentID)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		conn, err := tc.Connect(session, 80, 24)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	require.NoError(t, lastErr, "failed to open terminal after retries")
	return nil
}

func TestTerminalSession(T *testing.T) {
	agent := requireIdleAgent(T)

	T.Run("open session", func(t *testing.T) {
		termClient := getTerminalClient()
		session, err := termClient.OpenSession(agent.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, session.Token)

		// Attach once and close immediately so the session is fully torn down.
		conn, err := termClient.Connect(session, 80, 24)
		require.NoError(t, err)
		conn.Close()
	})

	T.Run("connect websocket", func(t *testing.T) {
		conn := openTerminalConn(t, agent.ID)

		ctx, cancel := context.WithTimeout(T.Context(), 5*time.Second)
		defer cancel()
		_ = conn.Exec(ctx, "true")

		conn.Close()
		conn.Close() // idempotent
	})

	T.Run("invalid agent", func(t *testing.T) {
		termClient := getTerminalClient()
		_, err := termClient.OpenSession(999999)
		require.Error(t, err)
	})
}

func TestTerminalExec(T *testing.T) {
	agent := requireIdleAgent(T)

	// Warm up the agent-terminal plugin — the first session after agent registration
	// can take 20-30s for the shell to spawn. If the warmup fails, the plugin is
	// wedged and all subtests would just hit their 30s timeouts.
	conn := openTerminalConn(T, agent.ID)
	ctx, cancel := context.WithTimeout(T.Context(), 60*time.Second)
	err := conn.Exec(ctx, "true")
	cancel()
	if err != nil {
		T.Skipf("agent-terminal warmup failed, skipping subtests: %v", err)
	}

	T.Run("simple command", func(t *testing.T) {
		conn := openTerminalConn(t, agent.ID)

		ctx, cancel := context.WithTimeout(T.Context(), 30*time.Second)
		defer cancel()

		err := conn.Exec(ctx, "echo hello-terminal")
		require.NoError(t, err)
	})

	T.Run("multiline with env vars", func(t *testing.T) {
		conn := openTerminalConn(t, agent.ID)

		ctx, cancel := context.WithTimeout(T.Context(), 30*time.Second)
		defer cancel()

		err := conn.Exec(ctx, "echo L1; echo L2; echo $HOME")
		require.NoError(t, err)
	})

	T.Run("pipes and special chars", func(t *testing.T) {
		conn := openTerminalConn(t, agent.ID)

		ctx, cancel := context.WithTimeout(T.Context(), 30*time.Second)
		defer cancel()

		err := conn.Exec(ctx, `echo "test" | tr 'a-z' 'A-Z'`)
		require.NoError(t, err)
	})

	T.Run("long output", func(t *testing.T) {
		conn := openTerminalConn(t, agent.ID)

		ctx, cancel := context.WithTimeout(T.Context(), 30*time.Second)
		defer cancel()

		err := conn.Exec(ctx, "seq 1 50")
		require.NoError(t, err)
	})

	T.Run("timeout", func(t *testing.T) {
		conn := openTerminalConn(t, agent.ID)

		ctx, cancel := context.WithTimeout(T.Context(), 500*time.Millisecond)
		defer cancel()

		err := conn.Exec(ctx, "sleep 2")
		require.Error(t, err)
	})
}
