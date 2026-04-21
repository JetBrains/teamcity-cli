package output

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestStatusIcon(T *testing.T) {
	T.Parallel()

	tests := []struct {
		status       string
		state        string
		statusText   string
		wantContains string
	}{
		{"SUCCESS", "", "", "✓"},
		{"FAILURE", "", "", "✗"},
		{"ERROR", "", "", "✗"},
		{"UNKNOWN", "", "", "?"},
		{"UNKNOWN", "", "Canceled (user)", "⊘"},
		{"OTHER", "", "", "○"},
		{"", "running", "", "●"},
		{"", "queued", "", "◦"},
		{"success", "", "", "✓"},
		{"failure", "", "", "✗"},
		{"Success", "", "", "✓"},
		{"Failure", "", "", "✗"},
		{"", "", "", "○"},
		{" ", "", "", "○"},
		{"SUCCESS", "", "Canceled", "✓"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state+"/"+tc.statusText, func(t *testing.T) {
			t.Parallel()

			got := ansi.Strip(StatusIcon(tc.status, tc.state, tc.statusText))
			assert.Contains(t, got, tc.wantContains)
		})
	}
}

func TestStatusText(T *testing.T) {
	T.Parallel()

	tests := []struct {
		status       string
		state        string
		statusText   string
		wantContains string
	}{
		{"SUCCESS", "", "", "Success"},
		{"FAILURE", "", "", "Failed"},
		{"ERROR", "", "", "Error"},
		{"UNKNOWN", "", "", "Unknown"},
		{"UNKNOWN", "", "Canceled", "Canceled"},
		{"", "running", "", "Running"},
		{"", "queued", "", "Queued"},
		{"OTHER", "", "", "OTHER"},
		{"SUCCESS", "", "Canceled", "Success"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state+"/"+tc.statusText, func(t *testing.T) {
			t.Parallel()

			got := ansi.Strip(StatusText(tc.status, tc.state, tc.statusText))
			assert.Contains(t, got, tc.wantContains)
		})
	}
}

func TestPlainStatusIcon(T *testing.T) {
	T.Parallel()
	tests := []struct {
		status     string
		state      string
		statusText string
		want       string
	}{
		{"SUCCESS", "", "", "+"},
		{"FAILURE", "", "", "x"},
		{"ERROR", "", "", "x"},
		{"UNKNOWN", "", "", "?"},
		{"UNKNOWN", "", "Canceled", "/"},
		{"OTHER", "", "-", "-"},
		{"", "running", "", "*"},
		{"", "queued", "", "o"},
		{"SUCCESS", "", "Canceled", "+"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state+"/"+tc.statusText, func(t *testing.T) {
			t.Parallel()
			got := PlainStatusIcon(tc.status, tc.state, tc.statusText)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPlainStatusText(T *testing.T) {
	T.Parallel()

	tests := []struct {
		status     string
		state      string
		statusText string
		want       string
	}{
		{"SUCCESS", "", "", "success"},
		{"FAILURE", "", "", "failure"},
		{"UNKNOWN", "", "Canceled", "canceled"},
		{"", "running", "", "running"},
		{"", "queued", "", "queued"},
		{"SUCCESS", "", "Canceled", "success"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state+"/"+tc.statusText, func(t *testing.T) {
			t.Parallel()

			got := PlainStatusText(tc.status, tc.state, tc.statusText)
			assert.Equal(t, tc.want, got)
		})
	}
}
