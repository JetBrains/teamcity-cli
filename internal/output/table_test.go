package output

import (
	"strings"
	"testing"

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

func TestAutoSizeColumns(T *testing.T) {
	T.Run("all data fits no truncation", func(t *testing.T) {
		overrideTerminal(t, true, 80, 24, nil)

		headers := []string{"A", "B", "C"}
		rows := [][]string{
			{"ID1", "short", "val"},
			{"ID2", "data", "ok"},
		}
		AutoSizeColumns(headers, rows, 2, 1)
		assert.Equal(t, "short", rows[0][1])
		assert.Equal(t, "data", rows[1][1])
	})

	T.Run("truncates overflowing flex column", func(t *testing.T) {
		overrideTerminal(t, true, 40, 24, nil)

		headers := []string{"A", "B", "C"}
		long := strings.Repeat("x", 50)
		rows := [][]string{
			{"ID", long, "end"},
		}
		AutoSizeColumns(headers, rows, 2, 1)
		assert.Less(t, len(rows[0][1]), 50)
		assert.Contains(t, rows[0][1], "...")
	})

	T.Run("short columns give space to long ones", func(t *testing.T) {
		overrideTerminal(t, true, 80, 24, nil)

		headers := []string{"F", "LONG", "B", "C"}
		rows := [][]string{
			{"F", strings.Repeat("a", 70), "bb", "cc"},
		}
		AutoSizeColumns(headers, rows, 2, 1, 2, 3)
		assert.Equal(t, "bb", rows[0][2])
		assert.Equal(t, "cc", rows[0][3])
		assert.GreaterOrEqual(t, len(rows[0][1]), 55, "long column should get most of the space")
	})

	T.Run("multiple overflowing columns split proportionally", func(t *testing.T) {
		overrideTerminal(t, true, 50, 24, nil)

		headers := []string{"X", "A", "B"}
		rows := [][]string{
			{"X", strings.Repeat("a", 80), strings.Repeat("b", 40)},
		}
		AutoSizeColumns(headers, rows, 2, 1, 2)
		w1 := len(rows[0][1])
		w2 := len(rows[0][2])
		assert.Greater(t, w1, w2, "wider-content column should get more space")
	})

	T.Run("empty rows is a no-op", func(t *testing.T) {
		AutoSizeColumns([]string{"A", "B"}, nil, 2, 0, 1)
		AutoSizeColumns([]string{}, [][]string{}, 2, 0)
	})

	T.Run("no flex cols is a no-op", func(t *testing.T) {
		rows := [][]string{{"a", "b"}}
		AutoSizeColumns([]string{"A", "B"}, rows, 2)
		assert.Equal(t, "a", rows[0][0])
		assert.Equal(t, "b", rows[0][1])
	})

	T.Run("fixed columns with ANSI keep correct width", func(t *testing.T) {
		overrideTerminal(t, true, 60, 24, nil)

		headers := []string{"STATUS", "DATA"}
		ansiRed := "\033[31mFailed\033[0m"
		rows := [][]string{
			{ansiRed, strings.Repeat("x", 80)},
		}
		AutoSizeColumns(headers, rows, 2, 1)
		assert.Equal(t, ansiRed, rows[0][0])
		assert.Contains(t, rows[0][1], "...")
	})

	T.Run("headers wider than data are accounted for", func(t *testing.T) {
		overrideTerminal(t, true, 50, 24, nil)

		// Fixed column header "TRIGGERED BY" (12 chars) is wider than data "vcs" (3 chars).
		// The function must reserve 12 chars for it, not 3.
		headers := []string{"TRIGGERED BY", "JOB"}
		rows := [][]string{
			{"vcs", strings.Repeat("j", 60)},
		}
		AutoSizeColumns(headers, rows, 2, 1)
		// JOB should get 50 - 12 - 2 = 36 chars (not 50 - 3 - 2 = 45)
		jobWidth := len(rows[0][1])
		assert.LessOrEqual(t, jobWidth, 38, "should account for header width of fixed column")
	})
}

func TestPrintJSON(T *testing.T) {
	T.Parallel()
	tests := []struct {
		name string
		data any
	}{
		{"map with string value", map[string]string{"key": "value"}},
		{"empty map", map[string]string{}},
		{"string slice", []string{"a", "b", "c"}},
		{"nested structure", map[string]any{"builds": []map[string]string{{"id": "1"}}}},
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
