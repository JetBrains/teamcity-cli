package api

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUserDate(T *testing.T) {
	T.Parallel()

	now := time.Now().UTC()

	tests := []struct {
		name       string
		input      string
		wantErr    bool
		validateFn func(t *testing.T, result string) bool
	}{
		{
			name:    "empty string returns empty",
			input:   "",
			wantErr: false,
			validateFn: func(t *testing.T, s string) bool {
				t.Helper()
				return s == ""
			},
		},
		{
			name:    "relative time 24h",
			input:   "24h",
			wantErr: false,
			validateFn: func(t *testing.T, s string) bool {
				t.Helper()
				parsed, err := ParseTeamCityTime(s)
				if err != nil {
					t.Logf("failed to parse result: %v", err)
					return false
				}
				expected := now.Add(-24 * time.Hour)
				diff := expected.Sub(parsed)
				return diff < time.Minute && diff > -time.Minute
			},
		},
		{
			name:    "relative time 48h",
			input:   "48h",
			wantErr: false,
			validateFn: func(t *testing.T, s string) bool {
				t.Helper()
				parsed, err := ParseTeamCityTime(s)
				if err != nil {
					t.Logf("failed to parse result: %v", err)
					return false
				}
				expected := now.Add(-48 * time.Hour)
				diff := expected.Sub(parsed)
				return diff < time.Minute && diff > -time.Minute
			},
		},
		{
			name:    "absolute date only",
			input:   "2026-01-21",
			wantErr: false,
			validateFn: func(t *testing.T, s string) bool {
				t.Helper()
				return strings.HasPrefix(s, "20260121")
			},
		},
		{
			name:    "absolute date and time",
			input:   "2026-01-21 15:04:05",
			wantErr: false,
			validateFn: func(t *testing.T, s string) bool {
				t.Helper()
				return strings.HasPrefix(s, "20260121T150405")
			},
		},
		{
			name:    "ISO8601 format",
			input:   "2026-01-21T15:04:05Z",
			wantErr: false,
			validateFn: func(t *testing.T, s string) bool {
				t.Helper()
				return strings.HasPrefix(s, "20260121T150405")
			},
		},
		{
			name:    "TeamCity format passthrough",
			input:   "20260121T150405+0000",
			wantErr: false,
			validateFn: func(t *testing.T, s string) bool {
				t.Helper()
				return s == "20260121T150405+0000"
			},
		},
		{
			name:    "invalid format returns error",
			input:   "notadate",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		T.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ParseUserDate(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.validateFn != nil {
				assert.True(t, tt.validateFn(t, result), "validation failed for input %q, got %q", tt.input, result)
			}
		})
	}
}

func TestFormatTeamCityTime(T *testing.T) {
	T.Parallel()
	testTime := time.Date(2026, 1, 21, 15, 4, 5, 0, time.UTC)
	got := FormatTeamCityTime(testTime)
	want := "20260121T150405+0000"

	assert.Equal(T, want, got)
}
