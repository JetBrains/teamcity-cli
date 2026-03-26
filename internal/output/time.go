package output

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
)

var shortTimeMagnitudes = []humanize.RelTimeMagnitude{
	{D: time.Minute, Format: "now", DivBy: time.Second},
	{D: 2 * time.Minute, Format: "1m ago", DivBy: 1},
	{D: time.Hour, Format: "%dm ago", DivBy: time.Minute},
	{D: 2 * time.Hour, Format: "1h ago", DivBy: 1},
	{D: 24 * time.Hour, Format: "%dh ago", DivBy: time.Hour},
	{D: 2 * 24 * time.Hour, Format: "1d ago", DivBy: 1},
	{D: 7 * 24 * time.Hour, Format: "%dd ago", DivBy: 24 * time.Hour},
}

// RelativeTime formats a time as relative to now
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	now := time.Now()
	if now.Sub(t) < 0 {
		return "now"
	}

	if now.Sub(t) >= 7*24*time.Hour {
		return t.Format("Jan 02")
	}

	return humanize.CustomRelTime(t, now, "", "", shortTimeMagnitudes)
}

// FormatDuration formats a duration in human-readable form
func FormatDuration(d time.Duration) string {
	if d < 0 {
		return "-"
	}

	if d < time.Second {
		return "< 1s"
	}

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", mins, secs)
	}

	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}
