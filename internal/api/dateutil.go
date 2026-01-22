package api

import (
	"fmt"
	"time"

	"github.com/araddon/dateparse"
)

// ParseUserDate converts user input to TeamCity date format.
// Supports: "24h" (relative), "2026-01-21" (absolute), or TeamCity format.
func ParseUserDate(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	if duration, err := time.ParseDuration(input); err == nil {
		targetTime := time.Now().UTC().Add(-duration)
		return FormatTeamCityTime(targetTime), nil
	}

	parsedTime, err := dateparse.ParseAny(input)
	if err == nil {
		return FormatTeamCityTime(parsedTime.UTC()), nil
	}

	if _, err := ParseTeamCityTime(input); err == nil {
		return input, nil
	}

	return "", fmt.Errorf("invalid date format: %s (expected formats: 24h, 2026-01-21, or 2026-01-21T15:04:05)", input)
}

// FormatTeamCityTime formats time to TeamCity's date format.
func FormatTeamCityTime(t time.Time) string {
	return t.Format("20060102T150405-0700")
}
