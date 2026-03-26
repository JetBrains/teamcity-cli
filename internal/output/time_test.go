package output

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
