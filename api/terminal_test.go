//go:build integration

package api_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTerminalClient() *api.TerminalClient {
	user, pass := os.Getenv("TEAMCITY_USER"), os.Getenv("TEAMCITY_PASSWORD")
	if user == "" {
		user, pass = "admin", "admin123"
	}
	return api.NewTerminalClient(client.BaseURL, user, pass)
}

func getTerminalAgent(t *testing.T) api.Agent {
	t.Helper()
	agents, err := client.GetAgents(api.AgentsOptions{})
	require.NoError(t, err)

	for _, a := range agents.Agents {
		if a.Connected && a.Authorized && a.Enabled {
			return a
		}
	}
	t.Fatal("no connected/authorized/enabled agent available")
	return api.Agent{}
}

func TestTerminalSession(T *testing.T) {
	agent := getTerminalAgent(T)

	T.Run("open session", func(t *testing.T) {
		termClient := getTerminalClient()
		session, err := termClient.OpenSession(agent.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, session.Token)
	})

	T.Run("connect websocket", func(t *testing.T) {
		termClient := getTerminalClient()
		session, err := termClient.OpenSession(agent.ID)
		require.NoError(t, err)

		conn, err := termClient.Connect(session, 80, 24)
		require.NoError(t, err)

		// Properly terminate the shell to avoid leaving server-side zombie sessions
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = conn.Exec(ctx, "true") // Run trivial command which sends exit

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
	agent := getTerminalAgent(T)

	T.Run("simple command", func(t *testing.T) {
		termClient := getTerminalClient()
		session, err := termClient.OpenSession(agent.ID)
		require.NoError(t, err)
		conn, err := termClient.Connect(session, 80, 24)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err = conn.Exec(ctx, "echo hello-terminal")
		require.NoError(t, err)
	})

	T.Run("multiline with env vars", func(t *testing.T) {
		termClient := getTerminalClient()
		session, err := termClient.OpenSession(agent.ID)
		require.NoError(t, err)
		conn, err := termClient.Connect(session, 80, 24)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err = conn.Exec(ctx, "echo L1; echo L2; echo $HOME")
		require.NoError(t, err)
	})

	T.Run("pipes and special chars", func(t *testing.T) {
		termClient := getTerminalClient()
		session, err := termClient.OpenSession(agent.ID)
		require.NoError(t, err)
		conn, err := termClient.Connect(session, 80, 24)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err = conn.Exec(ctx, `echo "test" | tr 'a-z' 'A-Z'`)
		require.NoError(t, err)
	})

	T.Run("long output", func(t *testing.T) {
		termClient := getTerminalClient()
		session, err := termClient.OpenSession(agent.ID)
		require.NoError(t, err)
		conn, err := termClient.Connect(session, 80, 24)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err = conn.Exec(ctx, "seq 1 50")
		require.NoError(t, err)
	})

	T.Run("timeout", func(t *testing.T) {
		termClient := getTerminalClient()
		session, err := termClient.OpenSession(agent.ID)
		require.NoError(t, err)
		conn, err := termClient.Connect(session, 80, 24)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err = conn.Exec(ctx, "sleep 10")
		require.Error(t, err)
	})
}
