package output

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

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

// isStdinTerminalFn is the function used to detect whether stdin is a terminal.
// Tests can override this to simulate interactive mode.
var isStdinTerminalFn = func() bool { return isatty.IsTerminal(os.Stdin.Fd()) }

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
			return nil, fmt.Errorf("PAGER is set but empty")
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
