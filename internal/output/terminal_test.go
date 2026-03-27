package output

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overrideTerminal sets isTerminalFn and getTermSizeFn for the duration of the test.
func overrideTerminal(t *testing.T, isTerm bool, w, h int, err error) {
	t.Helper()
	oldIsTerm := isTerminalFn
	oldGetSize := getTermSizeFn
	isTerminalFn = func() bool { return isTerm }
	getTermSizeFn = func() (int, int, error) { return w, h, err }
	t.Cleanup(func() {
		isTerminalFn = oldIsTerm
		getTermSizeFn = oldGetSize
	})
}

// captureStdout replaces os.Stdout with a pipe for the duration of fn and returns what was written.
// Reading happens concurrently to prevent deadlock when output exceeds the OS pipe buffer.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()

	fn()

	w.Close()
	os.Stdout = oldStdout
	<-done
	r.Close()
	return buf.String()
}

func TestTerminal(T *testing.T) {
	T.Parallel()
	T.Run("TerminalWidth", func(t *testing.T) {
		t.Parallel()

		got := TerminalWidth()
		assert.Greater(t, got, 0)
	})

	T.Run("TerminalSize", func(t *testing.T) {
		t.Parallel()

		w, h := TerminalSize()
		assert.Greater(t, w, 0)
		assert.Greater(t, h, 0)
	})

	T.Run("IsTerminal does not panic", func(t *testing.T) {
		t.Parallel()
		_ = IsTerminal()
	})

	T.Run("IsStdinTerminal does not panic", func(t *testing.T) {
		t.Parallel()
		_ = IsStdinTerminal()
	})
}

func TestDefaultFnBodies(T *testing.T) {
	// Exercise the default function literal bodies for coverage.
	// These only run in production; tests typically override them.
	_, _, _ = getTermSizeFn()

	// With real PATH: LookPath succeeds
	cmd, err := pagerCmdFn()
	if err == nil {
		assert.NotNil(T, cmd)
	}

	// With empty PATH: LookPath fails, covering the error branch
	T.Setenv("PATH", "")
	_, err = pagerCmdFn()
	assert.Error(T, err)
}

func TestTerminalSizeTerminal(T *testing.T) {
	overrideTerminal(T, true, 100, 50, nil)

	w, h := TerminalSize()
	assert.Equal(T, 100, w)
	assert.Equal(T, 50, h)
}

func TestTerminalSizeError(T *testing.T) {
	overrideTerminal(T, true, 0, 0, errors.New("not a terminal"))

	w, h := TerminalSize()
	assert.Equal(T, 80, w)
	assert.Equal(T, 24, h)
}

func TestTerminalSizeZeroWidth(T *testing.T) {
	overrideTerminal(T, true, 0, 50, nil)

	w, h := TerminalSize()
	assert.Equal(T, 80, w)
	assert.Equal(T, 24, h)
}

func TestWithPagerNonTerminal(T *testing.T) {
	overrideTerminal(T, false, 120, 40, nil)

	var buf bytes.Buffer
	WithPager(&buf, func(w io.Writer) {
		fmt.Fprintln(w, "hello pager")
	})
	assert.Contains(T, buf.String(), "hello pager")
}

func TestWithPagerFallbackShortContent(T *testing.T) {
	overrideTerminal(T, true, 80, 50, nil)

	var buf bytes.Buffer
	WithPager(&buf, func(w io.Writer) {
		fmt.Fprintln(w, "short content")
	})
	assert.Contains(T, buf.String(), "short content")
}

func TestWithPagerRunsLess(T *testing.T) {
	overrideTerminal(T, true, 80, 5, nil)

	// Generate content that exceeds terminal height
	var lines []string
	for i := range 20 {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n") + "\n"

	// less requires a real terminal for stdin, so it will fail and fall back to the out writer
	output := captureStdout(T, func() {
		WithPager(os.Stdout, func(w io.Writer) {
			fmt.Fprint(w, content)
		})
	})
	assert.Contains(T, output, "line 0")
}

func TestWithPagerLessError(T *testing.T) {
	overrideTerminal(T, true, 80, 5, nil)

	oldPager := pagerCmdFn
	T.Cleanup(func() { pagerCmdFn = oldPager })
	pagerCmdFn = func() (*exec.Cmd, error) {
		return exec.Command("false"), nil // "false" always exits 1
	}

	var lines []string
	for i := range 20 {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n") + "\n"

	var buf bytes.Buffer
	// pager fails → falls back to the out writer
	captureStdout(T, func() {
		WithPager(&buf, func(w io.Writer) {
			fmt.Fprint(w, content)
		})
	})
	assert.Contains(T, buf.String(), "line 0")
	assert.Contains(T, buf.String(), "line 19")
}
