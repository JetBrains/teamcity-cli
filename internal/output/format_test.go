package output

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/acarl005/stripansi"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "no truncation needed",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "truncate with ellipsis",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "very short max shows ellipsis",
			input:    "hello",
			maxLen:   3,
			expected: "...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   5,
			expected: "",
		},
		// Edge cases - runewidth.Truncate always appends "..." when truncating
		{
			name:     "maxLen 0",
			input:    "hello",
			maxLen:   0,
			expected: "...", // runewidth.Truncate appends ellipsis even at 0
		},
		{
			name:     "maxLen 1",
			input:    "hello",
			maxLen:   1,
			expected: "...", // runewidth.Truncate appends ellipsis
		},
		{
			name:     "maxLen 2",
			input:    "hello",
			maxLen:   2,
			expected: "...", // runewidth.Truncate appends ellipsis
		},
		{
			name:     "unicode characters",
			input:    "日本語テスト",
			maxLen:   8,
			expected: "日本...",
		},
		{
			name:     "emoji",
			input:    "🚀🎉🔥test",
			maxLen:   6,
			expected: "🚀...",
		},
		{
			name:     "single unicode char with truncate",
			input:    "日",
			maxLen:   5,
			expected: "日",
		},
		{
			name:     "string with newlines",
			input:    "hello\nworld",
			maxLen:   8,
			expected: "hello\n...", // runewidth counts newline as width 0
		},
		{
			name:     "negative maxLen",
			input:    "hello",
			maxLen:   -1,
			expected: "...", // runewidth.Truncate appends ellipsis
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Truncate(tc.input, tc.maxLen)
			if result != tc.expected {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.expected)
			}
		})
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status   string
		state    string
		contains string
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
		t.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			result := StatusIcon(tc.status, tc.state)
			stripped := stripansi.Strip(result)
			if !strings.Contains(stripped, tc.contains) {
				t.Errorf("StatusIcon(%q, %q) = %q, want to contain %q", tc.status, tc.state, stripped, tc.contains)
			}
		})
	}
}

func TestStatusText(t *testing.T) {
	tests := []struct {
		status   string
		state    string
		contains string
	}{
		{"SUCCESS", "", "Success"},
		{"FAILURE", "", "Failed"},
		{"ERROR", "", "Error"},
		{"", "running", "Running"},
		{"", "queued", "Queued"},
		{"OTHER", "", "OTHER"},
	}

	for _, tc := range tests {
		t.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			result := StatusText(tc.status, tc.state)
			stripped := stripansi.Strip(result)
			if !strings.Contains(stripped, tc.contains) {
				t.Errorf("StatusText(%q, %q) = %q, want to contain %q", tc.status, tc.state, stripped, tc.contains)
			}
		})
	}
}

func TestPlainStatusIcon(t *testing.T) {
	tests := []struct {
		status   string
		state    string
		expected string
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
		t.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			result := PlainStatusIcon(tc.status, tc.state)
			if result != tc.expected {
				t.Errorf("PlainStatusIcon(%q, %q) = %q, want %q", tc.status, tc.state, result, tc.expected)
			}
		})
	}
}

func TestPlainStatusText(t *testing.T) {
	tests := []struct {
		status   string
		state    string
		expected string
	}{
		{"SUCCESS", "", "success"},
		{"FAILURE", "", "failure"},
		{"", "running", "running"},
		{"", "queued", "queued"},
	}

	for _, tc := range tests {
		t.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			result := PlainStatusText(tc.status, tc.state)
			if result != tc.expected {
				t.Errorf("PlainStatusText(%q, %q) = %q, want %q", tc.status, tc.state, result, tc.expected)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "zero time",
			time:     time.Time{},
			expected: "-",
		},
		{
			name:     "just now",
			time:     now.Add(-30 * time.Second),
			expected: "now",
		},
		{
			name:     "1 minute ago",
			time:     now.Add(-1 * time.Minute),
			expected: "1m ago",
		},
		{
			name:     "5 minutes ago",
			time:     now.Add(-5 * time.Minute),
			expected: "5m ago",
		},
		{
			name:     "1 hour ago",
			time:     now.Add(-1 * time.Hour),
			expected: "1h ago",
		},
		{
			name:     "3 hours ago",
			time:     now.Add(-3 * time.Hour),
			expected: "3h ago",
		},
		{
			name:     "1 day ago",
			time:     now.Add(-24 * time.Hour),
			expected: "1d ago",
		},
		{
			name:     "3 days ago",
			time:     now.Add(-3 * 24 * time.Hour),
			expected: "3d ago",
		},
		{
			name:     "future time",
			time:     now.Add(1 * time.Hour),
			expected: "now",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RelativeTime(tc.time)
			if result != tc.expected {
				t.Errorf("RelativeTime() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestRelativeTimeOlderThanWeek(t *testing.T) {
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	result := RelativeTime(oldTime)
	if !strings.Contains(result, oldTime.Format("Jan")) {
		t.Errorf("RelativeTime for old date should show month, got %q", result)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "negative duration",
			duration: -1 * time.Second,
			expected: "-",
		},
		{
			name:     "zero duration",
			duration: 0,
			expected: "< 1s",
		},
		{
			name:     "milliseconds",
			duration: 500 * time.Millisecond,
			expected: "< 1s",
		},
		{
			name:     "seconds",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			expected: "2m 30s",
		},
		{
			name:     "hours and minutes",
			duration: 2*time.Hour + 15*time.Minute,
			expected: "2h 15m",
		},
		// Boundary tests
		{
			name:     "exactly 1 second",
			duration: 1 * time.Second,
			expected: "1s",
		},
		{
			name:     "exactly 1 minute",
			duration: 1 * time.Minute,
			expected: "1m 0s",
		},
		{
			name:     "exactly 1 hour",
			duration: 1 * time.Hour,
			expected: "1h 0m",
		},
		{
			name:     "59 seconds",
			duration: 59 * time.Second,
			expected: "59s",
		},
		{
			name:     "60 seconds equals 1 minute",
			duration: 60 * time.Second,
			expected: "1m 0s",
		},
		{
			name:     "large duration over 24 hours",
			duration: 25*time.Hour + 30*time.Minute,
			expected: "25h 30m",
		},
		{
			name:     "999 milliseconds",
			duration: 999 * time.Millisecond,
			expected: "< 1s",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatDuration(tc.duration)
			if result != tc.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tc.duration, result, tc.expected)
			}
		})
	}
}

func TestOutputFunctions(t *testing.T) {
	oldQuiet := Quiet
	oldVerbose := Verbose
	defer func() {
		Quiet = oldQuiet
		Verbose = oldVerbose
	}()

	for _, quiet := range []bool{true, false} {
		t.Run(fmt.Sprintf("quiet=%v", quiet), func(t *testing.T) {
			Quiet = quiet
			Success("test %s", "message")
			Info("test %s", "info")
			Infof("test %s", "infof")
			Warn("test %s", "warn")
		})
	}

	for _, verbose := range []bool{true, false} {
		t.Run(fmt.Sprintf("verbose=%v", verbose), func(t *testing.T) {
			Verbose = verbose
			Debug("test %s", "debug")
		})
	}
}

func TestColumnWidths(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			result := ColumnWidths(tc.margin, tc.minFlex, tc.percentages...)
			if len(result) != tc.wantLen {
				t.Errorf("got %d columns, want %d", len(result), tc.wantLen)
			}
			for _, w := range result {
				if w < 0 {
					t.Errorf("negative width: %d", w)
				}
			}
		})
	}
}

func TestTerminal(t *testing.T) {
	w := TerminalWidth()
	if w <= 0 {
		t.Errorf("TerminalWidth() = %d, want positive", w)
	}

	w, h := TerminalSize()
	if w <= 0 || h <= 0 {
		t.Errorf("TerminalSize() = (%d, %d), want positive", w, h)
	}

	_ = IsTerminal()
	_ = IsStdinTerminal()
}

func TestPrintJSON(t *testing.T) {
	cases := []interface{}{
		map[string]string{"key": "value"},
		map[string]string{},
		[]string{"a", "b", "c"},
		map[string]interface{}{"builds": []map[string]string{{"id": "1"}}},
	}
	for i, data := range cases {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			if err := PrintJSON(data); err != nil {
				t.Errorf("PrintJSON error: %v", err)
			}
		})
	}
}

func TestPrintTable(t *testing.T) {
	cases := []struct {
		headers []string
		rows    [][]string
	}{
		{[]string{"ID", "Name"}, [][]string{{"1", "Test"}, {"2", "Test2"}}},
		{[]string{}, [][]string{}},
		{[]string{"Status"}, [][]string{{"OK"}, {"FAIL"}}},
		{[]string{"Build", "Status"}, [][]string{{"🚀 Build", "✓"}}},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			PrintTable(tc.headers, tc.rows)
		})
	}
}

func TestPrintPlainTable(t *testing.T) {
	cases := []struct {
		headers  []string
		rows     [][]string
		noHeader bool
	}{
		{[]string{"ID", "Name"}, [][]string{{"1", "Test"}}, false},
		{[]string{"ID", "Name"}, [][]string{{"1", "Test"}}, true},
		{[]string{}, [][]string{}, false},
		{[]string{"A", "B"}, [][]string{{"1", "2", "3"}}, false},
		{[]string{"Name", "Status"}, [][]string{{"日本語", "✓"}}, false},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			PrintPlainTable(tc.headers, tc.rows, tc.noHeader)
		})
	}
}
