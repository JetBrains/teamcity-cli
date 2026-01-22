package output

import (
	"strings"
	"testing"
	"time"

	"github.com/acarl005/stripansi"
)

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "ascii only",
			input:    "hello",
			expected: 5,
		},
		{
			name:     "with ansi codes",
			input:    "\x1b[32mhello\x1b[0m",
			expected: 5,
		},
		{
			name:     "unicode checkmark",
			input:    "✓",
			expected: 1,
		},
		{
			name:     "unicode with ansi",
			input:    "\x1b[32m✓\x1b[0m",
			expected: 1,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "mixed unicode and ascii",
			input:    "✓ Success",
			expected: 9,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := displayWidth(tc.input)
			if result != tc.expected {
				t.Errorf("displayWidth(%q) = %d, want %d", tc.input, result, tc.expected)
			}
		})
	}
}

func TestPadToWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "pad short string",
			input:    "hi",
			width:    5,
			expected: "hi   ",
		},
		{
			name:     "no padding needed",
			input:    "hello",
			width:    5,
			expected: "hello",
		},
		{
			name:     "input longer than width",
			input:    "hello world",
			width:    5,
			expected: "hello world",
		},
		{
			name:     "with ansi codes",
			input:    "\x1b[32mhi\x1b[0m",
			width:    5,
			expected: "\x1b[32mhi\x1b[0m   ",
		},
		{
			name:     "empty string",
			input:    "",
			width:    3,
			expected: "   ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := padToWidth(tc.input, tc.width)
			if result != tc.expected {
				t.Errorf("padToWidth(%q, %d) = %q, want %q", tc.input, tc.width, result, tc.expected)
			}
		})
	}
}

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

func TestSuccessAndInfo(t *testing.T) {
	// Test with Quiet = true (should not print)
	oldQuiet := Quiet
	Quiet = true
	Success("test %s", "message")
	Info("test %s", "info")
	Infof("test %s", "infof")
	Warn("test %s", "warn")
	Quiet = oldQuiet
}

func TestDebug(t *testing.T) {
	// Test with Verbose = true
	oldVerbose := Verbose
	Verbose = true
	Debug("test %s", "debug")
	Verbose = oldVerbose
}

func TestColumnWidths(t *testing.T) {
	tests := []struct {
		name        string
		margin      int
		minFlex     int
		percentages []int
		wantLen     int
	}{
		{"single column", 20, 50, []int{100}, 1},
		{"two columns", 20, 50, []int{50, 50}, 2},
		{"three columns", 30, 60, []int{40, 30, 30}, 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ColumnWidths(tc.margin, tc.minFlex, tc.percentages...)
			if len(result) != tc.wantLen {
				t.Errorf("ColumnWidths() returned %d columns, want %d", len(result), tc.wantLen)
			}
		})
	}
}

func TestTerminalWidth(t *testing.T) {
	// TerminalWidth should return a positive value
	w := TerminalWidth()
	if w <= 0 {
		t.Errorf("TerminalWidth() = %d, want positive", w)
	}
}

func TestTerminalSize(t *testing.T) {
	w, h := TerminalSize()
	if w <= 0 || h <= 0 {
		t.Errorf("TerminalSize() = (%d, %d), want positive values", w, h)
	}
}

func TestIsTerminal(t *testing.T) {
	// Just ensure it doesn't panic
	_ = IsTerminal()
	_ = IsStdinTerminal()
}

func TestPrintJSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	err := PrintJSON(data)
	if err != nil {
		t.Errorf("PrintJSON() error = %v", err)
	}
}

func TestPrintTable(t *testing.T) {
	headers := []string{"ID", "Name", "Status"}
	rows := [][]string{
		{"1", "Build One", "Success"},
		{"2", "Build Two", "Failed"},
	}
	// Just ensure it doesn't panic
	PrintTable(headers, rows)
}

func TestPrintPlainTable(t *testing.T) {
	headers := []string{"ID", "Name"}
	rows := [][]string{
		{"1", "Test"},
		{"2", "Test2"},
	}
	PrintPlainTable(headers, rows, false)
	PrintPlainTable(headers, rows, true) // no header
}
