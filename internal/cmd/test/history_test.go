package test

import (
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
)

func occ(status string, durMS int) api.TestOccurrence {
	return api.TestOccurrence{Status: status, Duration: durMS}
}

func TestComputeTestStats(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		s := computeTestStats(nil)
		assert.Equal(t, 0, s.Considered)
		assert.Equal(t, 0.0, s.PassRate)
		assert.Equal(t, time.Duration(0), s.AvgDuration)
	})

	t.Run("all_pass", func(t *testing.T) {
		s := computeTestStats([]api.TestOccurrence{
			occ("SUCCESS", 1000),
			occ("SUCCESS", 3000),
		})
		assert.Equal(t, 2, s.Passed)
		assert.Equal(t, 0, s.Failed)
		assert.Equal(t, 2, s.Considered)
		assert.Equal(t, 100.0, s.PassRate)
		assert.Equal(t, 2*time.Second, s.AvgDuration)
	})

	t.Run("mixed_ignores_ignored", func(t *testing.T) {
		s := computeTestStats([]api.TestOccurrence{
			occ("SUCCESS", 2000),
			occ("FAILURE", 4000),
			occ("IGNORED", 0),
		})
		assert.Equal(t, 1, s.Passed)
		assert.Equal(t, 1, s.Failed)
		assert.Equal(t, 1, s.Ignored)
		assert.Equal(t, 3, s.Total)
		assert.Equal(t, 2, s.Considered)
		assert.Equal(t, 50.0, s.PassRate)
		// Average over the two non-ignored runs only.
		assert.Equal(t, 3*time.Second, s.AvgDuration)
	})
}

func TestHistoryFooter(t *testing.T) {
	assert.Contains(t, historyFooter(computeTestStats(nil)), "Pass rate: n/a")
	footer := historyFooter(computeTestStats([]api.TestOccurrence{occ("SUCCESS", 1000), occ("FAILURE", 1000)}))
	assert.Contains(t, footer, "Pass rate: 50% (1/2)")
	assert.Contains(t, footer, "Avg duration:")
}
