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
	ptyExitWait    = 30 * time.Second // grace period for the spawned binary to exit after remote EOF
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

// ptyProc bundles the pty console with the spawned `teamcity` process so
// callers can assert the process exited cleanly after the remote shell EOFs.
// Without the exit-status assertion, a subprocess that prints the expected
// output but crashes on teardown would still pass these tests.
type ptyProc struct {
	*expect.Console
	cmd     *exec.Cmd
	done    chan struct{}
	waitErr error
}

// AssertCleanExit waits for the spawned `teamcity` process to exit and fails
// the test if it either times out or returns a non-zero status. Call after
// `ExpectEOF`.
func (p *ptyProc) AssertCleanExit(t *testing.T) {
	t.Helper()
	select {
	case <-p.done:
		require.NoError(t, p.waitErr, "teamcity process exited with error")
	case <-time.After(ptyExitWait):
		t.Fatalf("teamcity process did not exit within %s after shell EOF", ptyExitWait)
	}
}

func ptySpawn(t *testing.T, args ...string) *ptyProc {
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

	// Master sees EOF only after every slave fd is closed; child has its own dup.
	require.NoError(t, c.Tty().Close())

	p := &ptyProc{Console: c, cmd: cmd, done: make(chan struct{})}
	go func() {
		p.waitErr = cmd.Wait()
		close(p.done)
	}()

	t.Cleanup(func() {
		select {
		case <-p.done:
			return
		default:
		}
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-p.done
	})
	return p
}

func TestTerminalPtyLinux(t *testing.T) {
	agentID := os.Getenv(ptyEnvLinuxAgent)
	if agentID == "" {
		t.Skipf("%s not set — bring up an agent via CLI_LinuxAgentTerminal and export its id", ptyEnvLinuxAgent)
	}

	t.Run("prompt echo exit", func(t *testing.T) {
		p := ptySpawn(t, "agent", "term", agentID)
		_, err := p.Expect(expect.String("$ "))
		require.NoError(t, err, "no shell prompt seen")
		_, err = p.SendLine("echo pty-linux-ok")
		require.NoError(t, err)
		_, err = p.Expect(expect.String("pty-linux-ok"))
		require.NoError(t, err)
		_, err = p.SendLine("exit")
		require.NoError(t, err)
		_, err = p.ExpectEOF()
		require.NoError(t, err, "process did not exit cleanly after remote exit")
		p.AssertCleanExit(t)
	})

	t.Run("survives idle over ping interval", func(t *testing.T) {
		if testing.Short() {
			t.Skip("75s idle — skipped under -short")
		}
		p := ptySpawn(t, "agent", "term", agentID)
		_, err := p.Expect(expect.String("$ "))
		require.NoError(t, err)
		time.Sleep(ptyIdleWait)
		_, err = p.SendLine("echo post-idle-ok")
		require.NoError(t, err)
		_, err = p.Expect(expect.String("post-idle-ok"))
		require.NoError(t, err, "session died during idle — pong handler not refreshing deadline")
		_, err = p.SendLine("exit")
		require.NoError(t, err)
		_, err = p.ExpectEOF()
		require.NoError(t, err)
		p.AssertCleanExit(t)
	})
}

func TestTerminalPtyWindows(t *testing.T) {
	agentID := os.Getenv(ptyEnvWindowsAgent)
	if agentID == "" {
		t.Skipf("%s not set — bring up an agent via CLI_WindowsAgentTerminal and export its id", ptyEnvWindowsAgent)
	}

	t.Run("powershell prompt echo exit", func(t *testing.T) {
		p := ptySpawn(t, "agent", "term", agentID)
		_, err := p.Expect(expect.String("PS "))
		require.NoError(t, err, "no PowerShell prompt seen")
		_, err = p.Send("Write-Host pty-ps-ok\r")
		require.NoError(t, err)
		_, err = p.Expect(expect.String("pty-ps-ok"))
		require.NoError(t, err, "Write-Host output not seen — Enter keystroke not submitting on PS")
		_, err = p.Send("exit\r")
		require.NoError(t, err)
		_, err = p.ExpectEOF()
		require.NoError(t, err, "process did not exit cleanly after PS exit")
		p.AssertCleanExit(t)
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
