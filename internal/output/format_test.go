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
	"time"

	"github.com/acarl005/stripansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncate(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "no truncation needed",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "truncate with ellipsis",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "very short max shows ellipsis",
			input:  "hello",
			maxLen: 3,
			want:   "...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 5,
			want:   "",
		},
		// Edge cases - runewidth.Truncate always appends "..." when truncating
		{
			name:   "maxLen 0",
			input:  "hello",
			maxLen: 0,
			want:   "...", // runewidth.Truncate appends ellipsis even at 0
		},
		{
			name:   "maxLen 1",
			input:  "hello",
			maxLen: 1,
			want:   "...", // runewidth.Truncate appends ellipsis
		},
		{
			name:   "maxLen 2",
			input:  "hello",
			maxLen: 2,
			want:   "...", // runewidth.Truncate appends ellipsis
		},
		{
			name:   "unicode characters",
			input:  "日本語テスト",
			maxLen: 8,
			want:   "日本...",
		},
		{
			name:   "emoji",
			input:  "🚀🎉🔥test",
			maxLen: 6,
			want:   "🚀...",
		},
		{
			name:   "single unicode char with truncate",
			input:  "日",
			maxLen: 5,
			want:   "日",
		},
		{
			name:   "string with newlines",
			input:  "hello\nworld",
			maxLen: 8,
			want:   "hello\n...", // runewidth counts newline as width 0
		},
		{
			name:   "negative maxLen",
			input:  "hello",
			maxLen: -1,
			want:   "...", // runewidth.Truncate appends ellipsis
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := Truncate(tc.input, tc.maxLen)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestStatusIcon(T *testing.T) {
	T.Parallel()

	tests := []struct {
		status       string
		state        string
		wantContains string
	}{
		{"SUCCESS", "", "✓"},
		{"FAILURE", "", "✗"},
		{"ERROR", "", "✗"},
		{"UNKNOWN", "", "?"},
		{"OTHER", "", "○"},
		{"", "running", "●"},
		{"", "queued", "◦"},
		// Case insensitivity tests
		{"success", "", "✓"},
		{"failure", "", "✗"},
		{"Success", "", "✓"},
		{"Failure", "", "✗"},
		// Empty and edge cases
		{"", "", "○"},
		{" ", "", "○"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			t.Parallel()

			got := stripansi.Strip(StatusIcon(tc.status, tc.state))
			assert.Contains(t, got, tc.wantContains)
		})
	}
}

func TestStatusText(T *testing.T) {
	T.Parallel()

	tests := []struct {
		status       string
		state        string
		wantContains string
	}{
		{"SUCCESS", "", "Success"},
		{"FAILURE", "", "Failed"},
		{"ERROR", "", "Error"},
		{"", "running", "Running"},
		{"", "queued", "Queued"},
		{"OTHER", "", "OTHER"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			t.Parallel()

			got := stripansi.Strip(StatusText(tc.status, tc.state))
			assert.Contains(t, got, tc.wantContains)
		})
	}
}

func TestPlainStatusIcon(T *testing.T) {
	T.Parallel()
	tests := []struct {
		status string
		state  string
		want   string
	}{
		{"SUCCESS", "", "+"},
		{"FAILURE", "", "x"},
		{"ERROR", "", "x"},
		{"UNKNOWN", "", "?"},
		{"OTHER", "", "-"},
		{"", "running", "*"},
		{"", "queued", "o"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			t.Parallel()
			got := PlainStatusIcon(tc.status, tc.state)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPlainStatusText(T *testing.T) {
	T.Parallel()

	tests := []struct {
		status string
		state  string
		want   string
	}{
		{"SUCCESS", "", "success"},
		{"FAILURE", "", "failure"},
		{"", "running", "running"},
		{"", "queued", "queued"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			t.Parallel()

			got := PlainStatusText(tc.status, tc.state)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRelativeTime(T *testing.T) {
	T.Parallel()
	now := time.Now()

	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{
			name: "zero time",
			time: time.Time{},
			want: "-",
		},
		{
			name: "just now",
			time: now.Add(-30 * time.Second),
			want: "now",
		},
		{
			name: "1 minute ago",
			time: now.Add(-1 * time.Minute),
			want: "1m ago",
		},
		{
			name: "5 minutes ago",
			time: now.Add(-5 * time.Minute),
			want: "5m ago",
		},
		{
			name: "1 hour ago",
			time: now.Add(-1 * time.Hour),
			want: "1h ago",
		},
		{
			name: "3 hours ago",
			time: now.Add(-3 * time.Hour),
			want: "3h ago",
		},
		{
			name: "1 day ago",
			time: now.Add(-24 * time.Hour),
			want: "1d ago",
		},
		{
			name: "3 days ago",
			time: now.Add(-3 * 24 * time.Hour),
			want: "3d ago",
		},
		{
			name: "future time",
			time: now.Add(1 * time.Hour),
			want: "now",
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := RelativeTime(tc.time)
			assert.Equal(t, tc.want, got)
		})
	}

	T.Run("older than a week shows date", func(t *testing.T) {
		t.Parallel()

		oldTime := time.Now().Add(-10 * 24 * time.Hour)
		got := RelativeTime(oldTime)
		assert.Contains(t, got, oldTime.Format("Jan"))
	})
}

func TestFormatDuration(T *testing.T) {
	T.Parallel()
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "negative duration",
			duration: -1 * time.Second,
			want:     "-",
		},
		{
			name:     "zero duration",
			duration: 0,
			want:     "< 1s",
		},
		{
			name:     "milliseconds",
			duration: 500 * time.Millisecond,
			want:     "< 1s",
		},
		{
			name:     "seconds",
			duration: 30 * time.Second,
			want:     "30s",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			want:     "2m 30s",
		},
		{
			name:     "hours and minutes",
			duration: 2*time.Hour + 15*time.Minute,
			want:     "2h 15m",
		},
		// Boundary tests
		{
			name:     "exactly 1 second",
			duration: 1 * time.Second,
			want:     "1s",
		},
		{
			name:     "exactly 1 minute",
			duration: 1 * time.Minute,
			want:     "1m 0s",
		},
		{
			name:     "exactly 1 hour",
			duration: 1 * time.Hour,
			want:     "1h 0m",
		},
		{
			name:     "59 seconds",
			duration: 59 * time.Second,
			want:     "59s",
		},
		{
			name:     "60 seconds equals 1 minute",
			duration: 60 * time.Second,
			want:     "1m 0s",
		},
		{
			name:     "large duration over 24 hours",
			duration: 25*time.Hour + 30*time.Minute,
			want:     "25h 30m",
		},
		{
			name:     "999 milliseconds",
			duration: 999 * time.Millisecond,
			want:     "< 1s",
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FormatDuration(tc.duration)
			assert.Equal(t, tc.want, got)
		})
	}
}

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

func TestOutputFunctions(T *testing.T) {
	// Cannot use T.Parallel() because this test modifies package-level Quiet/Verbose
	oldQuiet := Quiet
	oldVerbose := Verbose
	T.Cleanup(func() {
		Quiet = oldQuiet
		Verbose = oldVerbose
	})

	for _, quiet := range []bool{true, false} {
		T.Run(fmt.Sprintf("quiet=%v", quiet), func(t *testing.T) {
			Quiet = quiet
			Success("test %s", "message")
			Info("test %s", "info")
			Infof("test %s", "infof")
			Warn("test %s", "warn")
		})
	}

	for _, verbose := range []bool{true, false} {
		T.Run(fmt.Sprintf("verbose=%v", verbose), func(t *testing.T) {
			Verbose = verbose
			Debug("test %s", "debug")
		})
	}
}

func TestColumnWidths(T *testing.T) {
	T.Parallel()
	tests := []struct {
		name        string
		margin      int
		minFlex     int
		percentages []int
		wantLen     int
	}{
		{"single", 20, 50, []int{100}, 1},
		{"two", 20, 50, []int{50, 50}, 2},
		{"three", 30, 60, []int{40, 30, 30}, 3},
		{"large_margin", 10000, 50, []int{50, 50}, 2},
		{"zero_pct", 0, 0, []int{0, 0, 0}, 3},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ColumnWidths(tc.margin, tc.minFlex, tc.percentages...)
			assert.Equal(t, tc.wantLen, len(got))
			for i, w := range got {
				assert.GreaterOrEqual(t, w, 0, "ColumnWidths()[%d] should be non-negative", i)
			}
		})
	}
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

func TestPrintJSON(T *testing.T) {
	T.Parallel()
	tests := []struct {
		name string
		data interface{}
	}{
		{"map with string value", map[string]string{"key": "value"}},
		{"empty map", map[string]string{}},
		{"string slice", []string{"a", "b", "c"}},
		{"nested structure", map[string]interface{}{"builds": []map[string]string{{"id": "1"}}}},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := PrintJSON(tc.data)
			require.NoError(t, err)
		})
	}
}

func TestPrintTable(T *testing.T) {
	T.Parallel()
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
	}{
		{"basic table", []string{"ID", "Name"}, [][]string{{"1", "Test"}, {"2", "Test2"}}},
		{"empty", []string{}, [][]string{}},
		{"single column", []string{"Status"}, [][]string{{"OK"}, {"FAIL"}}},
		{"unicode content", []string{"Build", "Status"}, [][]string{{"🚀 Build", "✓"}}},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// PrintTable writes to stdout; just verify it doesn't panic
			PrintTable(tc.headers, tc.rows)
		})
	}
}

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

func TestPrintPlainTable(T *testing.T) {
	T.Parallel()
	tests := []struct {
		name     string
		headers  []string
		rows     [][]string
		noHeader bool
	}{
		{"with header", []string{"ID", "Name"}, [][]string{{"1", "Test"}}, false},
		{"without header", []string{"ID", "Name"}, [][]string{{"1", "Test"}}, true},
		{"empty", []string{}, [][]string{}, false},
		{"row longer than headers", []string{"A", "B"}, [][]string{{"1", "2", "3"}}, false},
		{"unicode content", []string{"Name", "Status"}, [][]string{{"日本語", "✓"}}, false},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// PrintPlainTable writes to stdout; just verify it doesn't panic
			PrintPlainTable(tc.headers, tc.rows, tc.noHeader)
		})
	}
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

	output := captureStdout(T, func() {
		WithPager(func(w io.Writer) {
			fmt.Fprintln(w, "hello pager")
		})
	})
	assert.Contains(T, output, "hello pager")
}

func TestWithPagerFallbackShortContent(T *testing.T) {
	overrideTerminal(T, true, 80, 50, nil)

	output := captureStdout(T, func() {
		WithPager(func(w io.Writer) {
			fmt.Fprintln(w, "short content")
		})
	})
	assert.Contains(T, output, "short content")
}

func TestWithPagerRunsLess(T *testing.T) {
	overrideTerminal(T, true, 80, 5, nil)

	// Generate content that exceeds terminal height
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n") + "\n"

	output := captureStdout(T, func() {
		WithPager(func(w io.Writer) {
			fmt.Fprint(w, content)
		})
	})
	// less requires a real terminal for stdin, so it will fail and fall back to direct write
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
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n") + "\n"

	output := captureStdout(T, func() {
		WithPager(func(w io.Writer) {
			fmt.Fprint(w, content)
		})
	})
	// pager fails → falls back to direct stdout write
	assert.Contains(T, output, "line 0")
	assert.Contains(T, output, "line 19")
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
