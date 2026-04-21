package api

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// userDateLayouts are the absolute-date formats accepted by ParseUserDate.
var userDateLayouts = []string{
	"2006-01-02",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	time.RFC3339,
	time.RFC3339Nano,
}

// ParseUserDate converts user input (duration like 24h/7d, ISO date, or TC format) to TeamCity date format.
func ParseUserDate(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	if duration, err := parseRelativeDuration(input); err == nil {
		if duration < 0 {
			return "", fmt.Errorf("negative duration not allowed: %s (use a positive value like 24h)", input)
		}
		return FormatTeamCityTime(time.Now().UTC().Add(-duration)), nil
	}

	for _, layout := range userDateLayouts {
		if t, err := time.Parse(layout, input); err == nil {
			return FormatTeamCityTime(t.UTC()), nil
		}
	}

	if _, err := ParseTeamCityTime(input); err == nil {
		return input, nil
	}

	return "", fmt.Errorf("invalid date: %s (expected duration like 24h/7d/2w or date like 2026-01-21)", input)
}

// FormatTeamCityTime formats time to TeamCity's date format.
func FormatTeamCityTime(t time.Time) string {
	return t.Format("20060102T150405-0700")
}

// parseRelativeDuration extends time.ParseDuration with d/w/mo/y units.
func parseRelativeDuration(input string) (time.Duration, error) {
	if input == "" {
		return 0, errors.New("empty duration")
	}

	var extended time.Duration
	var passthrough strings.Builder
	s := input

	for len(s) > 0 {
		i := 0
		for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
			i++
		}
		if i == 0 {
			return 0, fmt.Errorf("expected digit at %q", s)
		}
		numStr := s[:i]
		s = s[i:]

		j := 0
		for j < len(s) && ((s[j] >= 'a' && s[j] <= 'z') || (s[j] >= 'A' && s[j] <= 'Z')) {
			j++
		}
		if j == 0 {
			return 0, fmt.Errorf("missing unit after %q", numStr)
		}
		unit := s[:j]
		s = s[j:]

		var hours float64
		switch unit {
		case "y":
			hours = 365 * 24
		case "mo":
			hours = 30 * 24
		case "w":
			hours = 7 * 24
		case "d":
			hours = 24
		default:
			// Delegate Go-native units (ns/us/µs/ms/s/m/h) to time.ParseDuration.
			passthrough.WriteString(numStr)
			passthrough.WriteString(unit)
			continue
		}
		n, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number %q: %w", numStr, err)
		}
		extended += time.Duration(n * hours * float64(time.Hour))
	}

	if passthrough.Len() == 0 {
		return extended, nil
	}
	rest, err := time.ParseDuration(passthrough.String())
	if err != nil {
		return 0, err
	}
	return extended + rest, nil
}
