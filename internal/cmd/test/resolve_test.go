package test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseResolveState(t *testing.T) {
	t.Parallel()

	t.Run("fixed_default", func(t *testing.T) {
		for _, in := range []string{"", "fixed", "FIXED", " Fixed "} {
			got, err := parseResolveState(in)
			require.NoError(t, err)
			assert.Equal(t, "FIXED", got)
		}
	})

	t.Run("given_up", func(t *testing.T) {
		for _, in := range []string{"given-up", "GIVEN_UP", "givenup"} {
			got, err := parseResolveState(in)
			require.NoError(t, err)
			assert.Equal(t, "GIVEN_UP", got)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := parseResolveState("done")
		require.Error(t, err)
		var ve *api.ValidationError
		require.ErrorAs(t, err, &ve)
	})
}
