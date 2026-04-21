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

// overridePagerResolver sets PagerResolver for the duration of the test.
func overridePagerResolver(t *testing.T, pager string) {
	t.Helper()
	old := PagerResolver
	PagerResolver = func() string { return pager }
	t.Cleanup(func() { PagerResolver = old })
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

func TestWithPagerUsingNonTerminal(T *testing.T) {
	overrideTerminal(T, false, 120, 40, nil)

	var buf bytes.Buffer
	WithPagerUsing("less", PagerOpts{}, &buf, func(w io.Writer) {
		fmt.Fprintln(w, "hello pager")
	})
	assert.Contains(T, buf.String(), "hello pager")
}

func TestWithPagerUsingEmptyPagerSkipsPaging(T *testing.T) {
	overrideTerminal(T, true, 80, 5, nil)

	// Even long content with an empty pager goes straight to the writer.
	var lines []string
	for i := range 50 {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n") + "\n"

	var buf bytes.Buffer
	WithPagerUsing("", PagerOpts{}, &buf, func(w io.Writer) {
		fmt.Fprint(w, content)
	})
	assert.Contains(T, buf.String(), "line 49")
}

func TestWithPagerUsingCatIsNoOp(T *testing.T) {
	overrideTerminal(T, true, 80, 5, nil)

	var lines []string
	for i := range 50 {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n") + "\n"

	var buf bytes.Buffer
	WithPagerUsing("cat", PagerOpts{}, &buf, func(w io.Writer) {
		fmt.Fprint(w, content)
	})
	assert.Contains(T, buf.String(), "line 49")

	var buf2 bytes.Buffer
	WithPagerUsing("/bin/cat -u", PagerOpts{}, &buf2, func(w io.Writer) {
		fmt.Fprint(w, content)
	})
	assert.Contains(T, buf2.String(), "line 49")
}

func TestWithPagerUsingFallbackShortContent(T *testing.T) {
	overrideTerminal(T, true, 80, 50, nil)

	var buf bytes.Buffer
	WithPagerUsing("less", PagerOpts{}, &buf, func(w io.Writer) {
		fmt.Fprintln(w, "short content")
	})
	assert.Contains(T, buf.String(), "short content")
}

// TestWithPagerUsingEarlyExitPager covers the gh-style fix: a pager that
// reads only part of its stdin and exits 0 (e.g. less with `q` before
// consuming everything) must NOT cause the full buffer to be re-dumped
// to stdout after paging finishes.
func TestWithPagerUsingEarlyExitPager(T *testing.T) {
	overrideTerminal(T, true, 80, 5, nil)

	if _, err := exec.LookPath("head"); err != nil {
		T.Skip("head not available")
	}

	var lines []string
	for i := range 5000 {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n") + "\n"

	var fallback bytes.Buffer
	out := captureStdout(T, func() {
		WithPagerUsing("head -c 100", PagerOpts{}, &fallback, func(w io.Writer) {
			fmt.Fprint(w, content)
		})
	})

	assert.NotEmpty(T, out, "pager should have written something to stdout")
	assert.Empty(T, fallback.String(), "fallback buffer must not be written to when pager exits 0 after partial read")
}

// TestWithPagerUsingNonZeroExitFallsBack covers a misconfigured pager:
// the binary exists and Start() succeeds, but the pager exits non-zero
// before displaying anything (bad flag, unsupported option, etc.). We
// must re-dump the buffer so the user doesn't see a blank terminal.
func TestWithPagerUsingNonZeroExitFallsBack(T *testing.T) {
	overrideTerminal(T, true, 80, 5, nil)

	if _, err := exec.LookPath("sh"); err != nil {
		T.Skip("sh not available")
	}

	var lines []string
	for i := range 20 {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n") + "\n"

	var buf bytes.Buffer
	// `sh -c 'exit 1'` starts cleanly then exits non-zero without reading
	// stdin — mimics a misconfigured pager.
	WithPagerUsing("sh -c exit_1", PagerOpts{}, &buf, func(w io.Writer) {
		fmt.Fprint(w, content)
	})
	// buildPagerCmd's Fields split gives us `sh -c exit_1`, where `exit_1`
	// is an unknown command to sh → sh exits 127. Either way the exit is
	// non-zero and the fallback must fire.
	assert.Contains(T, buf.String(), "line 0")
	assert.Contains(T, buf.String(), "line 19")
}

// TestWithPagerUsingStartFailureFallsBack: if the pager binary can't be
// located, fall back to a direct write.
func TestWithPagerUsingStartFailureFallsBack(T *testing.T) {
	overrideTerminal(T, true, 80, 5, nil)

	var lines []string
	for i := range 20 {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n") + "\n"

	var buf bytes.Buffer
	WithPagerUsing("definitely-not-a-real-pager-binary-xyz", PagerOpts{}, &buf, func(w io.Writer) {
		fmt.Fprint(w, content)
	})
	assert.Contains(T, buf.String(), "line 0")
	assert.Contains(T, buf.String(), "line 19")
}

func TestWithPagerUsesResolver(T *testing.T) {
	overrideTerminal(T, false, 120, 40, nil) // non-terminal → short-circuit to writer
	overridePagerResolver(T, "less -R")

	var buf bytes.Buffer
	WithPager(&buf, func(w io.Writer) {
		fmt.Fprintln(w, "log body")
	})
	assert.Contains(T, buf.String(), "log body")
}

func TestWithPagerEmptyResolverUsesDefault(T *testing.T) {
	overrideTerminal(T, false, 120, 40, nil)
	overridePagerResolver(T, "")

	// Non-terminal makes WithPagerUsing skip paging and write directly; this
	// still exercises the default-pager fallback path in WithPager without
	// actually invoking a pager process.
	var buf bytes.Buffer
	WithPager(&buf, func(w io.Writer) {
		fmt.Fprintln(w, "log body")
	})
	assert.Contains(T, buf.String(), "log body")
}

func TestIsPagingDisabled(T *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"  ", true},
		{"cat", true},
		{"cat -u", true},
		{"/bin/cat", true},
		{"/usr/local/bin/cat --flag", true},
		{"less", false},
		{"less -FIRX", false},
		{"more", false},
		{"/usr/bin/less", false},
	}
	for _, tc := range cases {
		assert.Equal(T, tc.want, isPagingDisabled(tc.in), "input=%q", tc.in)
	}
}

func TestHasLessChopFlag(T *testing.T) {
	assert.True(T, hasLessChopFlag([]string{"-S"}))
	assert.True(T, hasLessChopFlag([]string{"-FIRSX"}))
	assert.True(T, hasLessChopFlag([]string{"--chop-long-lines"}))
	assert.False(T, hasLessChopFlag([]string{"-FIRX"}))
	assert.False(T, hasLessChopFlag(nil))
	assert.False(T, hasLessChopFlag([]string{"--mouse", "--incsearch"}))
}

func TestBuildPagerCmdAddsChopFlag(T *testing.T) {
	if _, err := exec.LookPath("less"); err != nil {
		T.Skip("less not available")
	}
	cmd, err := buildPagerCmd("less -FIRX", PagerOpts{ChopLongLines: true})
	require.NoError(T, err)
	require.NotNil(T, cmd)
	// The first arg injected should be -S; remaining args preserved.
	assert.Equal(T, []string{"-S", "-FIRX"}, cmd.Args[1:])
}

func TestBuildPagerCmdRespectsExistingChopFlag(T *testing.T) {
	if _, err := exec.LookPath("less"); err != nil {
		T.Skip("less not available")
	}
	cmd, err := buildPagerCmd("less -FIRSX", PagerOpts{ChopLongLines: true})
	require.NoError(T, err)
	require.NotNil(T, cmd)
	assert.Equal(T, []string{"-FIRSX"}, cmd.Args[1:])
}

func TestBuildPagerCmdSkipsChopForNonLess(T *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		T.Skip("cat not available")
	}
	cmd, err := buildPagerCmd("cat -u", PagerOpts{ChopLongLines: true})
	require.NoError(T, err)
	require.NotNil(T, cmd)
	assert.Equal(T, []string{"-u"}, cmd.Args[1:])
}
