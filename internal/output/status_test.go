package output

import (
	"testing"

	"github.com/acarl005/stripansi"
	"github.com/stretchr/testify/assert"
)

func TestStatusIcon(T *testing.T) {
	T.Parallel()

	tests := []struct {
		status       string
		state        string
		wantContains string
	}{
		{"SUCCESS", "", "✓"},
		{"FAILURE", "", "✗"},
		{"ERROR", "", "✗"},
		{"UNKNOWN", "", "?"},
		{"OTHER", "", "○"},
		{"", "running", "●"},
		{"", "queued", "◦"},
		// Case insensitivity tests
		{"success", "", "✓"},
		{"failure", "", "✗"},
		{"Success", "", "✓"},
		{"Failure", "", "✗"},
		// Empty and edge cases
		{"", "", "○"},
		{" ", "", "○"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			t.Parallel()

			got := stripansi.Strip(StatusIcon(tc.status, tc.state))
			assert.Contains(t, got, tc.wantContains)
		})
	}
}

func TestStatusText(T *testing.T) {
	T.Parallel()

	tests := []struct {
		status       string
		state        string
		wantContains string
	}{
		{"SUCCESS", "", "Success"},
		{"FAILURE", "", "Failed"},
		{"ERROR", "", "Error"},
		{"", "running", "Running"},
		{"", "queued", "Queued"},
		{"OTHER", "", "OTHER"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			t.Parallel()

			got := stripansi.Strip(StatusText(tc.status, tc.state))
			assert.Contains(t, got, tc.wantContains)
		})
	}
}

func TestPlainStatusIcon(T *testing.T) {
	T.Parallel()
	tests := []struct {
		status string
		state  string
		want   string
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
		T.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			t.Parallel()
			got := PlainStatusIcon(tc.status, tc.state)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPlainStatusText(T *testing.T) {
	T.Parallel()

	tests := []struct {
		status string
		state  string
		want   string
	}{
		{"SUCCESS", "", "success"},
		{"FAILURE", "", "failure"},
		{"", "running", "running"},
		{"", "queued", "queued"},
	}

	for _, tc := range tests {
		T.Run(tc.status+"/"+tc.state, func(t *testing.T) {
			t.Parallel()

			got := PlainStatusText(tc.status, tc.state)
			assert.Equal(t, tc.want, got)
		})
	}
}
