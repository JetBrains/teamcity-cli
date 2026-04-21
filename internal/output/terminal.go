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

// defaultLongFormPager is the fallback pager for long-form content
// (`run log`, `run diff`) when neither TEAMCITY_PAGER, the config `pager`
// key, nor the PAGER env var is set. It matches gh's less defaults plus
// `-R` for ANSI colors and `-X` to avoid clearing the screen on exit.
const defaultLongFormPager = "less -FIRX --mouse --incsearch"

// PagerResolver returns the pager command string (as the user would type it).
// The main package wires this to config.ResolvePager so the output package
// doesn't need to import config. An empty return value means paging is
// disabled; callers decide whether to fall back to a default pager.
var PagerResolver = func() string { return "" }

// PagerOpts tunes pager invocation.
type PagerOpts struct {
	// ChopLongLines asks the pager not to wrap lines. Used for wide tables
	// where wrapping breaks column alignment. Only applied when the resolved
	// pager is `less` (or unspecified, i.e. the default less fallback).
	ChopLongLines bool
}

// WithPager pipes long-form output through the resolved pager, defaulting to
// less when nothing is configured. Used by `run log` and `run diff`, which
// auto-page regardless of user config.
func WithPager(out io.Writer, fn func(w io.Writer)) {
	cmd := PagerResolver()
	if cmd == "" {
		cmd = defaultLongFormPager
	}
	WithPagerUsing(cmd, PagerOpts{}, out, fn)
}

// WithPagerUsing pipes output through the given pager command. An empty
// pagerCmd (or one that resolves to `cat`) writes directly to out. When the
// content is short enough to fit on screen, it also writes directly — no
// reason to invoke a pager for a few lines.
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
	// Stdout/Stderr must be the real terminal fds; using out here breaks
	// pager rendering when out is a buffer.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_, _ = out.Write(buf.Bytes())
		return
	}

	// Copy the buffer into the pager's stdin. Ignore the io.Copy error
	// because an early quit closes the pipe and we'd see EPIPE; the exit
	// status from Wait is the authoritative signal.
	_, _ = io.Copy(stdin, bytes.NewReader(buf.Bytes()))
	_ = stdin.Close()
	// A non-zero pager exit almost always means a misconfiguration (bad
	// flag, missing command in the pager's argv, etc.) — `less` / `more`
	// / `bat` all exit 0 on `q`, so a normal user quit doesn't land here.
	// Fall back to a direct write so the user doesn't get a blank
	// terminal. The edge case where a user Ctrl+C's the pager after
	// partial read will double-display, which is annoying but strictly
	// better than silently dropping their output.
	if err := cmd.Wait(); err != nil {
		_, _ = out.Write(buf.Bytes())
	}
}

// isPagingDisabled returns true when the resolved pager should be treated
// as a no-op: empty, or `cat` (a common way to disable paging via PAGER=cat).
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

// buildPagerCmd turns a user-facing pager string like `less -R` into an
// *exec.Cmd. When the resolved pager's basename is `less` and ChopLongLines
// is requested, `-S` is added if the user didn't already specify it.
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

// hasLessChopFlag reports whether the user already passed -S (or --chop-long-lines)
// to less. Matches `-S` alone, combined short flags like `-FIRSX`, and the long
// form `--chop-long-lines`.
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
