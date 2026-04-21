package output

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// isTerminalFn returns whether stdout is a terminal; tests override to force modes.
var isTerminalFn = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// getTermSizeFn returns the terminal size; tests override to return fixed values.
var getTermSizeFn = func() (int, int, error) { return term.GetSize(int(os.Stdout.Fd())) }

// IsTerminal returns true if stdout is a terminal.
func IsTerminal() bool {
	return isTerminalFn()
}

// isStdinTerminalFn returns whether stdin is a terminal; tests override to force modes.
var isStdinTerminalFn = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// IsStdinTerminal returns true if stdin is a terminal
func IsStdinTerminal() bool {
	return isStdinTerminalFn()
}

// TerminalSize returns terminal width and height (defaults: 80x24)
func TerminalSize() (int, int) {
	if !IsTerminal() {
		return 120, 40
	}
	width, height, err := getTermSizeFn()
	if err != nil || width <= 0 {
		return 80, 24
	}
	return width, height
}

// TerminalWidth returns the terminal width, or 80 as default
func TerminalWidth() int {
	w, _ := TerminalSize()
	return w
}

// pagerCmdFn creates the pager command. Tests can override this.
var pagerCmdFn = func() (*exec.Cmd, error) {
	if pager := os.Getenv("PAGER"); pager != "" {
		parts := strings.Fields(pager)
		if len(parts) == 0 {
			return nil, errors.New("PAGER is set but empty")
		}
		bin, err := exec.LookPath(parts[0])
		if err != nil {
			return nil, err
		}
		return exec.Command(bin, parts[1:]...), nil
	}
	lessPath, err := exec.LookPath("less")
	if err != nil {
		return nil, err
	}
	return exec.Command(lessPath, "-FIRX", "--mouse", "--incsearch"), nil
}

// WithPager pipes output through less if it exceeds terminal height.
// The out writer is used as a fallback when paging is not available.
func WithPager(out io.Writer, fn func(w io.Writer)) {
	var buf bytes.Buffer
	fn(&buf)

	_, height := TerminalSize()
	lineCount := bytes.Count(buf.Bytes(), []byte{'\n'})
	pager, err := pagerCmdFn()

	if !IsTerminal() || err != nil || lineCount <= height-2 {
		_, _ = out.Write(buf.Bytes())
		return
	}

	data := buf.Bytes()
	pager.Stdin = bytes.NewReader(data)
	pager.Stdout = os.Stdout // must be a real terminal fd; using `out` here breaks pager rendering
	pager.Stderr = os.Stderr
	if err := pager.Run(); err != nil {
		_, _ = out.Write(data)
	}
}
