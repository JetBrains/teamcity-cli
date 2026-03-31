package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"0.5.0", "0.6.0", true},
		{"0.6.0", "0.6.0", false},
		{"0.7.0", "0.6.0", false},
		{"0.5.0", "1.0.0", true},
		{"1.0.0", "0.9.0", false},
		{"0.5.0", "0.5.1", true},
		{"0.5.1", "0.5.0", false},
		{"v0.5.0", "v0.6.0", true},
		{"0.5.0-rc1", "0.5.0", false},
		{"dev", "0.5.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.current+"_vs_"+tt.latest, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNewer(tt.current, tt.latest))
		})
	}
}
