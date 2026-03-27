package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintLogo(T *testing.T) {
	var buf bytes.Buffer
	PrintLogo(&buf)

	output := buf.String()
	assert.NotEmpty(T, output, "logo output should not be empty")
}

func TestPrintLogoTerminal(T *testing.T) {
	overrideTerminal(T, true, 80, 24, nil)

	var buf bytes.Buffer
	PrintLogo(&buf)

	// Terminal animation should contain ANSI escape sequences
	assert.Contains(T, buf.String(), "\033[", "should contain ANSI escape sequences")
	assert.NotEmpty(T, buf.String(), "logo output should not be empty")
}
