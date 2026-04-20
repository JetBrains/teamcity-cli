//go:build terminal_pty

// Drives `teamcity agent term` under a real pty against live agents on
// cli.teamcity.com. Agent ids come from TC_PTY_{LINUX,WINDOWS}_AGENT_ID.
package api_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	expect "github.com/Netflix/go-expect"
	"github.com/stretchr/testify/require"
)

const (
	ptyEnvBinary       = "TC_PTY_BINARY"
	ptyEnvHost         = "TC_PTY_HOST"
	ptyEnvToken        = "TC_PTY_TOKEN"
	ptyEnvLinuxAgent   = "TC_PTY_LINUX_AGENT_ID"
	ptyEnvWindowsAgent = "TC_PTY_WINDOWS_AGENT_ID"

	ptyDefaultHost = "https://cli.teamcity.com"
	ptyIdleWait    = 75 * time.Second // > pingInterval (60s) so a missed pong would be visible
)

func ptyBinary(t *testing.T) string {
	t.Helper()
	if bin := os.Getenv(ptyEnvBinary); bin != "" {
		abs, err := filepath.Abs(bin)
		require.NoError(t, err)
		return abs
	}
	bin := filepath.Join(t.TempDir(), "teamcity")
	cmd := exec.Command("go", "build", "-o", bin, "./tc")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build: %s", out)
	return bin
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

func ptyEnv() []string {
	host := os.Getenv(ptyEnvHost)
	if host == "" {
		host = ptyDefaultHost
	}
	env := append(os.Environ(), "TEAMCITY_URL="+host)
	token := os.Getenv(ptyEnvToken)
	if token == "" {
		token = os.Getenv("TEAMCITY_TOKEN")
	}
	if token != "" {
		env = append(env, "TEAMCITY_TOKEN="+token)
	}
	return env
}

func ptySpawn(t *testing.T, args ...string) *expect.Console {
	t.Helper()
	c, err := expect.NewConsole(expect.WithDefaultTimeout(45 * time.Second))
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	cmd := exec.Command(ptyBinary(t), args...)
	cmd.Env = ptyEnv()
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()
	require.NoError(t, cmd.Start())

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})
	return c
}

func TestTerminalPtyLinux(t *testing.T) {
	agentID := os.Getenv(ptyEnvLinuxAgent)
	if agentID == "" {
		t.Skipf("%s not set — bring up an agent via CLI_LinuxAgentTerminal and export its id", ptyEnvLinuxAgent)
	}

	t.Run("prompt echo exit", func(t *testing.T) {
		c := ptySpawn(t, "agent", "term", agentID)
		_, err := c.Expect(expect.String("$ "))
		require.NoError(t, err, "no shell prompt seen")
		_, err = c.SendLine("echo pty-linux-ok")
		require.NoError(t, err)
		_, err = c.Expect(expect.String("pty-linux-ok"))
		require.NoError(t, err)
		_, err = c.SendLine("exit")
		require.NoError(t, err)
		_, err = c.ExpectEOF()
		require.NoError(t, err, "process did not exit cleanly after remote exit")
	})

	t.Run("survives idle over ping interval", func(t *testing.T) {
		if testing.Short() {
			t.Skip("75s idle — skipped under -short")
		}
		c := ptySpawn(t, "agent", "term", agentID)
		_, err := c.Expect(expect.String("$ "))
		require.NoError(t, err)
		time.Sleep(ptyIdleWait)
		_, err = c.SendLine("echo post-idle-ok")
		require.NoError(t, err)
		_, err = c.Expect(expect.String("post-idle-ok"))
		require.NoError(t, err, "session died during idle — pong handler not refreshing deadline")
		_, err = c.SendLine("exit")
		require.NoError(t, err)
		_, err = c.ExpectEOF()
		require.NoError(t, err)
	})
}

func TestTerminalPtyWindows(t *testing.T) {
	agentID := os.Getenv(ptyEnvWindowsAgent)
	if agentID == "" {
		t.Skipf("%s not set — bring up an agent via CLI_WindowsAgentTerminal and export its id", ptyEnvWindowsAgent)
	}

	t.Run("powershell prompt echo exit", func(t *testing.T) {
		c := ptySpawn(t, "agent", "term", agentID)
		_, err := c.Expect(expect.String("PS "))
		require.NoError(t, err, "no PowerShell prompt seen")
		_, err = c.SendLine("Write-Host pty-ps-ok")
		require.NoError(t, err)
		_, err = c.Expect(expect.String("pty-ps-ok"))
		require.NoError(t, err, "Write-Host output not seen — Enter keystroke not submitting on PS")
		_, err = c.SendLine("exit")
		require.NoError(t, err)
		_, err = c.ExpectEOF()
		require.NoError(t, err, "process did not exit cleanly after PS exit")
	})
}

// Regresses the deadline-scope decision: Exec must not inherit the
// interactive read deadline, or silent long commands time out before ctx.
func TestTerminalPtyExecSilentLong(t *testing.T) {
	if testing.Short() {
		t.Skip("170s exec — skipped under -short")
	}
	agentID := os.Getenv(ptyEnvLinuxAgent)
	if agentID == "" {
		t.Skipf("%s not set", ptyEnvLinuxAgent)
	}

	cmd := exec.Command(ptyBinary(t), "agent", "exec", agentID,
		"sleep 170; echo done-after-170s",
		"--timeout", "4m")
	cmd.Env = ptyEnv()
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "silent long exec failed: %s", out)
	require.Contains(t, string(out), "done-after-170s")
}
