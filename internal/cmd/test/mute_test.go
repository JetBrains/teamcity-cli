package test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUntil(t *testing.T) {
	t.Parallel()

	t.Run("permanent_default", func(t *testing.T) {
		for _, in := range []string{"", "permanent", "PERMANENT", " Permanent "} {
			res, err := parseUntil(in)
			require.NoError(t, err)
			assert.Equal(t, "manually", res.Type)
			assert.Empty(t, res.Time)
		}
	})

	t.Run("fixed", func(t *testing.T) {
		res, err := parseUntil("fixed")
		require.NoError(t, err)
		assert.Equal(t, "whenFixed", res.Type)
		assert.Empty(t, res.Time)
	})

	t.Run("date", func(t *testing.T) {
		res, err := parseUntil("2026-01-21")
		require.NoError(t, err)
		assert.Equal(t, "atTime", res.Type)
		assert.Equal(t, "20260121T000000+0000", res.Time)
	})

	t.Run("datetime", func(t *testing.T) {
		res, err := parseUntil("2026-01-21T18:30:00")
		require.NoError(t, err)
		assert.Equal(t, "atTime", res.Type)
		assert.Equal(t, "20260121T183000+0000", res.Time)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := parseUntil("next tuesday")
		require.Error(t, err)
		var ve *api.ValidationError
		require.ErrorAs(t, err, &ve)
	})
}
