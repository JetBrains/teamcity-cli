//go:build integration || terminal_pty

// Drives `teamcity agent term` and `teamcity agent exec` under a real pty,
// against whichever server the integration testenv brought up. Per-OS
// subtests skip cleanly when no agent of that family is connected.
package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	expect "github.com/Netflix/go-expect"
	"github.com/stretchr/testify/require"

	"github.com/JetBrains/teamcity-cli/api"
)

const (
	ptyIdleWait = 75 * time.Second // > pingInterval (60s) so a missed pong would surface
	ptyExitWait = 30 * time.Second // grace period for the spawned binary to exit after remote EOF
)

func ptyBinary(t *testing.T) string {
	t.Helper()
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

// ptyEnv hands the spawned `teamcity` process the same URL+token the test process uses.
func ptyEnv() []string {
	env := os.Environ()
	if testEnvRef != nil && testEnvRef.URL != "" {
		env = append(env, "TEAMCITY_URL="+testEnvRef.URL)
		if testEnvRef.Token != "" {
			env = append(env, "TEAMCITY_TOKEN="+testEnvRef.Token)
		}
	}
	return env
}

// pickAgentByOS returns the first authorized+connected agent whose teamcity.agent.os.family matches, or "" with a skip reason.
func pickAgentByOS(t *testing.T, osFamily string) (id, skip string) {
	t.Helper()

	if testEnvRef == nil || testEnvRef.Client == nil {
		return "", "no integration env (set TEAMCITY_URL/TEAMCITY_TOKEN or have Docker for testcontainers)"
	}
	if testEnvRef.guestAuth {
		return "", "guest auth lacks CONNECT_TO_AGENT permission"
	}

	agents, err := testEnvRef.Client.GetAgents(api.AgentsOptions{})
	if err != nil {
		return "", fmt.Sprintf("could not list agents: %v", err)
	}

	want := strings.ToLower(osFamily)
	for _, a := range agents.Agents {
		if !a.Connected || !a.Authorized {
			continue
		}
		family, err := agentOSFamily(t.Context(), testEnvRef.URL, testEnvRef.Token, a.ID)
		if err != nil {
			t.Logf("agent %d: could not read os family: %v", a.ID, err)
			continue
		}
		if strings.Contains(strings.ToLower(family), want) {
			return strconv.Itoa(a.ID), ""
		}
	}
	return "", fmt.Sprintf("no authorized %s agent connected", osFamily)
}

// agentOSFamily reads a single agent's teamcity.agent.os.family property via a narrow field selector.
func agentOSFamily(ctx context.Context, serverURL, token string, agentID int) (string, error) {
	url := fmt.Sprintf("%s/app/rest/agents/id:%d?fields=properties(property(name,value))",
		strings.TrimSuffix(serverURL, "/"), agentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("agent %d: HTTP %d", agentID, resp.StatusCode)
	}

	var body struct {
		Properties struct {
			Property []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"property"`
		} `json:"properties"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	for _, p := range body.Properties.Property {
		if p.Name == "teamcity.agent.os.family" {
			return p.Value, nil
		}
	}
	return "", nil // property absent — caller treats as "no match"
}

// ptyProc bundles the pty console with the spawned `teamcity` process so
// callers can assert clean exit. Without that assertion, a subprocess that
// prints the right output but crashes on teardown would still pass.
type ptyProc struct {
	*expect.Console
	cmd     *exec.Cmd
	done    chan struct{}
	waitErr error
}

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
	agentID, skip := pickAgentByOS(t, "Linux")
	if skip != "" {
		t.Skipf("Linux PTY skipped: %s", skip)
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
	agentID, skip := pickAgentByOS(t, "Windows")
	if skip != "" {
		t.Skipf("Windows PTY skipped: %s", skip)
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

// TestTerminalPtyExecSilentLong regresses the deadline-scope decision: Exec must not inherit the interactive read deadline.
func TestTerminalPtyExecSilentLong(t *testing.T) {
	if testing.Short() {
		t.Skip("170s exec — skipped under -short")
	}
	agentID, skip := pickAgentByOS(t, "Linux")
	if skip != "" {
		t.Skipf("exec long skipped: %s", skip)
	}

	cmd := exec.Command(ptyBinary(t), "agent", "exec", agentID,
		"sleep 170; echo done-after-170s",
		"--timeout", "4m")
	cmd.Env = ptyEnv()
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "silent long exec failed: %s", out)
	require.Contains(t, string(out), "done-after-170s")
}
