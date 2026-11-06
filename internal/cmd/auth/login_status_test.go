//go:build !windows

package auth_test

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserLoginProgramStatus(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		name := "terminal"
		if jsonOutput {
			name = "json"
		}
		t.Run(name, func(t *testing.T) {
			ts := newWebLoginServer(t, true)
			approveInBrowser(t, nil)
			t.Setenv("TERM", "xterm-256color")
			t.Setenv("CI", "")
			t.Setenv("TEAMCITY_VERSION", "")
			master, terminal, err := pty.Open()
			require.NoError(t, err)
			defer master.Close()
			defer terminal.Close()
			var stdout bytes.Buffer
			ts.Factory.Printer = &output.Printer{Out: &stdout, ErrOut: terminal}
			root := cmd.NewCommand(ts.Factory)
			args := []string{"auth", "login", "--server", ts.URL, "--web", "--insecure-storage"}
			if jsonOutput {
				args = append(args, "--json")
			}
			root.SetArgs(args)
			require.NoError(t, root.Execute())
			_, err = terminal.Write([]byte{0})
			require.NoError(t, err)
			reports, err := bufio.NewReader(master).ReadString(0)
			require.NoError(t, err)
			if jsonOutput {
				assert.NotContains(t, reports, "7501;")
			} else {
				assert.Regexp(t, `(?s)7501;state=working.*7501;state=blocked:app=teamcity:kind=auth.*7501;state=working.*7501;state=done`, reports)
			}
			assert.NotContains(t, stdout.String(), "7501;")
		})
	}
}
