package api

import (
	"strings"
	"testing"
	"time"
)

func TestParseUserDate(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		validateFn  func(string) bool
		description string
	}{
		{
			name:        "empty string",
			input:       "",
			wantErr:     false,
			validateFn:  func(s string) bool { return s == "" },
			description: "should return empty string for empty input",
		},
		{
			name:    "relative time - 24 hours",
			input:   "24h",
			wantErr: false,
			validateFn: func(s string) bool {
				parsed, err := ParseTeamCityTime(s)
				if err != nil {
					return false
				}
				expected := now.Add(-24 * time.Hour)
				diff := expected.Sub(parsed)
				// Allow 1 minute tolerance
				return diff < time.Minute && diff > -time.Minute
			},
			description: "should parse 24h as 24 hours ago",
		},
		{
			name:    "relative time - 48 hours",
			input:   "48h",
			wantErr: false,
			validateFn: func(s string) bool {
				parsed, err := ParseTeamCityTime(s)
				if err != nil {
					return false
				}
				expected := now.Add(-48 * time.Hour)
				diff := expected.Sub(parsed)
				return diff < time.Minute && diff > -time.Minute
			},
			description: "should parse 48h as 48 hours ago",
		},
		{
			name:    "absolute date - date only",
			input:   "2026-01-21",
			wantErr: false,
			validateFn: func(s string) bool {
				// dateparse should handle this - check it starts with the date
				return strings.HasPrefix(s, "20260121")
			},
			description: "should parse 2026-01-21",
		},
		{
			name:    "absolute date - date and time",
			input:   "2026-01-21 15:04:05",
			wantErr: false,
			validateFn: func(s string) bool {
				return strings.HasPrefix(s, "20260121T150405")
			},
			description: "should parse 2026-01-21 15:04:05 correctly",
		},
		{
			name:    "absolute date - ISO8601",
			input:   "2026-01-21T15:04:05Z",
			wantErr: false,
			validateFn: func(s string) bool {
				return strings.HasPrefix(s, "20260121T150405")
			},
			description: "should parse ISO8601 format correctly",
		},
		{
			name:    "TeamCity format passthrough",
			input:   "20260121T150405+0000",
			wantErr: false,
			validateFn: func(s string) bool {
				return s == "20260121T150405+0000"
			},
			description: "should pass through TeamCity format unchanged",
		},
		{
			name:        "invalid format",
			input:       "notadate",
			wantErr:     true,
			description: "should return error for invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseUserDate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseUserDate() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseUserDate() unexpected error = %v", err)
				return
			}

			if tt.validateFn != nil && !tt.validateFn(result) {
				t.Errorf("ParseUserDate() = %v, validation failed: %s", result, tt.description)
			}
		})
	}
}

func TestFormatTeamCityTime(t *testing.T) {
	testTime := time.Date(2026, 1, 21, 15, 4, 5, 0, time.UTC)
	result := FormatTeamCityTime(testTime)

	expected := "20260121T150405+0000"
	if result != expected {
		t.Errorf("FormatTeamCityTime() = %v, want %v", result, expected)
	}
}
