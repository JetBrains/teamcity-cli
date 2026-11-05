//go:build !windows

package output

import (
	"bytes"
	"io"
	"testing"

	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgramStatusTerminal(t *testing.T) {
	master, terminal, err := pty.Open()
	require.NoError(t, err)
	defer master.Close()
	defer terminal.Close()
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("CI", "")
	t.Setenv("TEAMCITY_VERSION", "")
	var stdout bytes.Buffer
	p := &Printer{Out: &stdout, ErrOut: stopWriter{terminal}, Quiet: true}
	expected := programStatus(ProgramDone, "", "finished")
	p.ProgramStatus(ProgramDone, "finished")
	got := make([]byte, len(expected))
	_, err = io.ReadFull(master, got)
	require.NoError(t, err)
	assert.Equal(t, expected, string(got))
	assert.Empty(t, stdout.String())

	for _, key := range []string{"TERM", "CI", "TEAMCITY_VERSION"} {
		t.Run(key, func(t *testing.T) {
			value := "true"
			if key == "TERM" {
				value = "dumb"
			}
			t.Setenv(key, value)
			p.ProgramStatus(ProgramError, "must not appear")
			_, err := terminal.Write([]byte("sentinel"))
			require.NoError(t, err)
			got := make([]byte, len("sentinel"))
			_, err = io.ReadFull(master, got)
			require.NoError(t, err)
			assert.Equal(t, "sentinel", string(got))
		})
	}
}

func TestProgramWaitRestoresStatus(t *testing.T) {
	master, terminal, err := pty.Open()
	require.NoError(t, err)
	defer master.Close()
	defer terminal.Close()
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("CI", "")
	t.Setenv("TEAMCITY_VERSION", "")
	p := &Printer{ErrOut: terminal}
	p.ProgramStatus(ProgramWorking, "Signing in")
	restoreAuth := p.ProgramWait("auth", "Browser sign-in")
	restoreQuestion := p.ProgramWait("question", "Choose a server")
	restoreQuestion()
	restoreAuth()
	expected := programStatus(ProgramWorking, "", "Signing in") +
		programStatus("blocked", "auth", "Browser sign-in") +
		programStatus("blocked", "question", "Choose a server") +
		programStatus("blocked", "auth", "Browser sign-in") +
		programStatus(ProgramWorking, "", "Signing in")
	got := make([]byte, len(expected))
	_, err = io.ReadFull(master, got)
	require.NoError(t, err)
	assert.Equal(t, expected, string(got))

	p.SuppressProgramStatus = true
	restore := p.ProgramWait("auth", "must not appear")
	restore()
	_, err = terminal.Write([]byte("sentinel"))
	require.NoError(t, err)
	got = make([]byte, len("sentinel"))
	_, err = io.ReadFull(master, got)
	require.NoError(t, err)
	assert.Equal(t, "sentinel", string(got))
}
