package output

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
)

func TestTestCountsSummary(t *testing.T) {
	NoColor = true
	tests := []struct {
		name string
		in   api.TestOccurrences
		want string
	}{
		{"all zero", api.TestOccurrences{}, ""},
		{"only passed", api.TestOccurrences{Passed: 5}, "5 passed"},
		{"failed and muted", api.TestOccurrences{Failed: 2, Muted: 1}, "2 failed, 1 muted"},
		{"full", api.TestOccurrences{Passed: 10, Failed: 2, Muted: 1, Ignored: 3}, "10 passed, 2 failed, 1 muted, 3 ignored"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, TestCountsSummary(&tc.in))
		})
	}
}
