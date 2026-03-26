package output

import (
	"bytes"
	"io"
	"os"
	"os/exec"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

// isTerminalFn is the function used to detect whether stdout is a terminal.
// Tests can override this to simulate terminal mode.
var isTerminalFn = func() bool { return isatty.IsTerminal(os.Stdout.Fd()) }

// getTermSizeFn is the function used to get the terminal size.
// Tests can override this to return controlled values.
var getTermSizeFn = func() (int, int, error) { return term.GetSize(int(os.Stdout.Fd())) }

// IsTerminal returns true if stdout is a terminal
func IsTerminal() bool {
	return isTerminalFn()
}

// IsStdinTerminal returns true if stdin is a terminal
func IsStdinTerminal() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
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
	lessPath, err := exec.LookPath("less")
	if err != nil {
		return nil, err
	}
	return exec.Command(lessPath, "-FIRX", "--mouse", "--incsearch"), nil
}

// WithPager pipes output through less if it exceeds terminal height
func WithPager(fn func(w io.Writer)) {
	var buf bytes.Buffer
	fn(&buf)

	_, height := TerminalSize()
	lineCount := bytes.Count(buf.Bytes(), []byte{'\n'})
	pager, err := pagerCmdFn()

	if !IsTerminal() || err != nil || lineCount <= height-2 {
		_, _ = os.Stdout.Write(buf.Bytes())
		return
	}

	data := buf.Bytes()
	pager.Stdin = bytes.NewReader(data)
	pager.Stdout = os.Stdout
	pager.Stderr = os.Stderr
	if err := pager.Run(); err != nil {
		_, _ = os.Stdout.Write(data)
	}
}
