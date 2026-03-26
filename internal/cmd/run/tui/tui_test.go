package tui

import (
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/acarl005/stripansi"
	"github.com/stretchr/testify/assert"
)

func TestWatchModelRenderHeader(t *testing.T) {
	t.Parallel()

	t.Run("nil build shows refreshing", func(t *testing.T) {
		t.Parallel()
		got := watchModel{}.renderHeader()
		assert.Contains(t, got, "Refreshing")
	})

	t.Run("running build shows percentage", func(t *testing.T) {
		t.Parallel()
		got := watchModel{build: &api.Build{
			ID: 1, Number: "42", BuildTypeID: "X", State: "running", Status: "SUCCESS",
			WebURL: "https://tc/1", PercentageComplete: 45,
		}}.renderHeader()
		assert.Contains(t, got, "45%")
		assert.Contains(t, got, "#42")
	})

	t.Run("finished build hides percentage", func(t *testing.T) {
		t.Parallel()
		got := watchModel{build: &api.Build{
			ID: 1, Number: "42", BuildTypeID: "X", State: "finished", Status: "SUCCESS",
			WebURL: "https://tc/1", PercentageComplete: 100,
		}}.renderHeader()
		assert.NotContains(t, got, "100%")
	})

	t.Run("prefers BuildType.Name over BuildTypeID", func(t *testing.T) {
		t.Parallel()
		got := stripansi.Strip(watchModel{build: &api.Build{
			ID: 1, Number: "42", BuildTypeID: "FallbackID", State: "running", Status: "SUCCESS",
			WebURL: "https://tc/1", BuildType: &api.BuildType{Name: "Pretty Name"},
		}}.renderHeader())
		assert.Contains(t, got, "Pretty Name")
		assert.NotContains(t, got, "FallbackID")
	})

	t.Run("falls back to BuildTypeID when BuildType nil", func(t *testing.T) {
		t.Parallel()
		got := watchModel{build: &api.Build{
			ID: 1, Number: "42", BuildTypeID: "FallbackID", State: "running", Status: "SUCCESS",
			WebURL: "https://tc/1",
		}}.renderHeader()
		assert.Contains(t, got, "FallbackID")
	})
}

func TestWatchModelRenderLogs(t *testing.T) {
	t.Parallel()

	t.Run("empty logs shows waiting", func(t *testing.T) {
		t.Parallel()
		got := watchModel{width: 80, height: 24}.renderLogs(10)
		assert.Contains(t, got, "Waiting for logs")
	})

	t.Run("tails to last N lines", func(t *testing.T) {
		t.Parallel()
		lines := make([]string, 20)
		for i := range lines {
			lines[i] = "[12:00:00] line-" + string(rune('A'+i))
		}
		got := (watchModel{width: 80, height: 24, logLines: lines}).renderLogs(5)
		// height=5 → should show lines[15:] = P,Q,R,S,T
		assert.Contains(t, got, "line-P")
		assert.Contains(t, got, "line-T")
		assert.NotContains(t, got, "line-A")
		assert.NotContains(t, got, "line-O")
	})

	t.Run("fewer lines than height shows all", func(t *testing.T) {
		t.Parallel()
		got := (watchModel{
			width: 80, height: 24,
			logLines: []string{"[12:00:00] alpha", "[12:00:01] beta"},
		}).renderLogs(10)
		assert.Contains(t, got, "alpha")
		assert.Contains(t, got, "beta")
	})

	t.Run("long lines truncated to width", func(t *testing.T) {
		t.Parallel()
		long := "[12:00:00] " + strings.Repeat("x", 200)
		got := (watchModel{
			width: 50, height: 24,
			logLines: []string{long},
		}).renderLogs(5)
		assert.Contains(t, got, "...")
		assert.Less(t, len(got), len(long), "output should be shorter than input")
	})
}

func TestWatchModelView(t *testing.T) {
	t.Parallel()

	t.Run("error state", func(t *testing.T) {
		t.Parallel()
		m := watchModel{err: assert.AnError, width: 80, height: 24}
		got := m.View()
		assert.Contains(t, got, "Error:")
	})

	t.Run("zero dimensions shows refreshing", func(t *testing.T) {
		t.Parallel()
		got := watchModel{}.View()
		assert.Equal(t, "Refreshing...", got)
	})

	t.Run("normal state includes header and quit hint", func(t *testing.T) {
		t.Parallel()
		m := watchModel{
			width: 80, height: 24,
			build: &api.Build{
				ID: 1, Number: "1", BuildTypeID: "X", State: "running", Status: "SUCCESS",
				WebURL: "https://tc/1",
			},
			logLines: []string{"[12:00:00] hello"},
		}
		got := m.View()
		assert.Contains(t, got, "q quit")
		assert.Contains(t, got, "hello")
	})

	t.Run("finished build hides spinner in footer", func(t *testing.T) {
		t.Parallel()
		m := watchModel{
			width: 80, height: 24,
			build: &api.Build{
				ID: 1, Number: "1", BuildTypeID: "X", State: "finished", Status: "SUCCESS",
				WebURL: "https://tc/1",
			},
		}
		got := stripansi.Strip(m.View())
		// spinner tick character should not appear after "q quit" for finished builds
		parts := got[len(got)-20:]
		assert.NotContains(t, parts, "/")
	})
}

func TestFormatWatchLogLine(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard log line with step",
			input: "[10:30:45] : [Step 1/3] Compiling sources",
			want:  "[10:30:45] [Step 1/3] Compiling sources",
		},
		{
			name:  "too short",
			input: "[short]",
			want:  "",
		},
		{
			name:  "no opening bracket",
			input: "plain text without timestamp",
			want:  "",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "close bracket at position 8 passes",
			input: "[1234567]rest",
			want:  "[1234567] rest",
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, formatWatchLogLine(tc.input))
		})
	}
}

func TestParseWatchLogLines(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"filters empty lines", "\n\n\n", 0},
		{"filters export and exec", "export FOO=bar\nexec /bin/sh\n", 0},
		{"filters Current time", "Current time: 2026-01-01 10:00:00", 0},
		{"keeps valid log line", "[10:30:45] : [Step 1/1] Hello\r\n", 1},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Len(t, parseWatchLogLines(tc.input), tc.want)
		})
	}
}
