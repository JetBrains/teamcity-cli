package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func setTerminal(t *testing.T, v bool) {
	prev := isTerminalFn
	isTerminalFn = func() bool { return v }
	t.Cleanup(func() { isTerminalFn = prev })
}

func spinnerActive() bool {
	spin.mu.Lock()
	defer spin.mu.Unlock()
	return spin.active
}

func TestStartSpinnerNoOpOnDumbTerminal(t *testing.T) {
	setTerminal(t, true)
	t.Setenv("TERM", "dumb")

	StartSpinner(false)
	t.Cleanup(StopSpinner)
	assert.False(t, spinnerActive(), "dumb terminals cannot render \\r animation or the ANSI clear-line escape")
}

func TestStartSpinnerNoOpWhenQuietOrNonTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")

	setTerminal(t, false)
	StartSpinner(false)
	assert.False(t, spinnerActive())

	setTerminal(t, true)
	StartSpinner(true)
	t.Cleanup(StopSpinner)
	assert.False(t, spinnerActive())
}
