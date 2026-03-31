package cmdtest

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/require"
)

// CaptureOutput executes a CLI command and returns the combined stdout/stderr.
func CaptureOutput(t *testing.T, f *cmdutil.Factory, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	f.Printer = &output.Printer{Out: &buf, ErrOut: &buf}

	rootCmd := cmd.NewRootCmdWithFactory(f)
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	require.NoError(t, err, "Execute(%v)", args)
	return buf.String()
}

// CaptureErr executes a CLI command, asserts it errors, and returns the error.
func CaptureErr(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	var buf bytes.Buffer
	f.Printer = &output.Printer{Out: &buf, ErrOut: &buf}

	rootCmd := cmd.NewRootCmdWithFactory(f)
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	require.Error(t, err, "expected error for Execute(%v)", args)
	return err
}

// Dedent strips the common leading whitespace from a multi-line string.
// Leading/trailing blank lines are also trimmed. This allows writing
// expected output indented inside test functions.
func Dedent(s string) string {
	s = strings.TrimRight(s, " \t\n")
	if len(s) > 0 && s[0] == '\n' {
		s = s[1:]
	}

	lines := strings.Split(s, "\n")
	minIndent := math.MaxInt
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent == math.MaxInt {
		minIndent = 0
	}

	for i, line := range lines {
		if len(line) >= minIndent {
			lines[i] = line[minIndent:]
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
