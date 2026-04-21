package output

import (
	"bytes"
	"errors"
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

// defaultLongFormPager is the built-in pager for `run log` / `run diff` when nothing is configured.
const defaultLongFormPager = "less -FIRX --mouse --incsearch"

// PagerResolver returns the pager command string; wired by the main package so output doesn't import config.
var PagerResolver = func() string { return "" }

// PagerOpts tunes pager invocation.
type PagerOpts struct {
	// ChopLongLines injects `-S` when the resolved pager is `less`, keeping wide tables aligned.
	ChopLongLines bool
}

// WithPager pipes long-form output (logs, diffs) through the resolved pager, defaulting to less.
func WithPager(out io.Writer, fn func(w io.Writer)) {
	cmd := PagerResolver()
	if cmd == "" {
		cmd = defaultLongFormPager
	}
	WithPagerUsing(cmd, PagerOpts{}, out, fn)
}

// WithPagerUsing pipes output through pagerCmd; empty, `cat`, short content, or a non-terminal writes directly.
func WithPagerUsing(pagerCmd string, opts PagerOpts, out io.Writer, fn func(w io.Writer)) {
	if isPagingDisabled(pagerCmd) || !IsTerminal() {
		fn(out)
		return
	}

	var buf bytes.Buffer
	fn(&buf)

	_, height := TerminalSize()
	lineCount := bytes.Count(buf.Bytes(), []byte{'\n'})
	if lineCount <= height-2 {
		_, _ = out.Write(buf.Bytes())
		return
	}

	cmd, err := buildPagerCmd(pagerCmd, opts)
	if err != nil {
		_, _ = out.Write(buf.Bytes())
		return
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		_, _ = out.Write(buf.Bytes())
		return
	}
	// Stdout/Stderr must be real terminal fds; using out here breaks pager rendering when out is a buffer.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_, _ = out.Write(buf.Bytes())
		return
	}

	// Ignore copy errors: an early pager quit closes the pipe (EPIPE); Wait's exit code is authoritative.
	_, _ = io.Copy(stdin, bytes.NewReader(buf.Bytes()))
	_ = stdin.Close()
	// Non-zero exit → misconfigured pager (bad flag, missing argv); fall back so the user isn't left with a blank terminal. `q` exits 0 on less/more/bat, so normal quits don't land here.
	if err := cmd.Wait(); err != nil {
		_, _ = out.Write(buf.Bytes())
	}
}

// isPagingDisabled treats empty and `cat` (PAGER=cat is a common explicit disable) as no-ops.
func isPagingDisabled(pagerCmd string) bool {
	pagerCmd = strings.TrimSpace(pagerCmd)
	if pagerCmd == "" {
		return true
	}
	parts := strings.Fields(pagerCmd)
	if len(parts) == 0 {
		return true
	}
	base := parts[0]
	if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
		base = base[i+1:]
	}
	return base == "cat"
}

// buildPagerCmd parses `less -R`-style pager strings into *exec.Cmd and injects -S for less+ChopLongLines.
func buildPagerCmd(pagerCmd string, opts PagerOpts) (*exec.Cmd, error) {
	parts := strings.Fields(pagerCmd)
	if len(parts) == 0 {
		return nil, errors.New("pager command is empty")
	}

	bin, err := exec.LookPath(parts[0])
	if err != nil {
		return nil, err
	}

	args := parts[1:]
	if opts.ChopLongLines && isLessBinary(parts[0]) && !hasLessChopFlag(args) {
		args = append([]string{"-S"}, args...)
	}

	return exec.Command(bin, args...), nil
}

func isLessBinary(cmd string) bool {
	base := cmd
	if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".exe")
	return base == "less"
}

// hasLessChopFlag matches -S alone, combined short flags like -FIRSX, and --chop-long-lines.
func hasLessChopFlag(args []string) bool {
	for _, a := range args {
		if a == "--chop-long-lines" {
			return true
		}
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a, 'S') {
			return true
		}
	}
	return false
}
