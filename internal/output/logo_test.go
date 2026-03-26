package output

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintLogo(T *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(T, err)

	os.Stdout = w
	PrintLogo()
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	r.Close()

	output := buf.String()
	assert.NotEmpty(T, output, "logo output should not be empty")
}

func TestPrintLogoTerminal(T *testing.T) {
	overrideTerminal(T, true, 80, 24, nil)

	output := captureStdout(T, func() {
		PrintLogo()
	})

	// Terminal animation should contain ANSI escape sequences
	assert.Contains(T, output, "\033[", "should contain ANSI escape sequences")
	assert.NotEmpty(T, output, "logo output should not be empty")
}
