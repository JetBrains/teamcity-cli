package auth

import (
	"errors"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExchangeErrorRejectedSentinel(t *testing.T) {
	tests := []struct {
		status   int
		rejected bool
	}{
		{http.StatusUnauthorized, true},
		{http.StatusBadRequest, false},
	}
	for _, tc := range tests {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			err := exchangeError(api.ErrorFromBody(tc.status, nil), "https://tc.example")

			assert.Equal(t, tc.rejected, errors.Is(err, ErrExchangeRejected))
			ve, ok := errors.AsType[*api.ValidationError](err)
			require.True(t, ok, "still a *api.ValidationError for rendering, got %T", err)
			assert.NotEmpty(t, ve.Suggestion())
			_, ok = errors.AsType[api.UserError](err)
			assert.True(t, ok, "root Execute classifies it as a UserError")
		})
	}
}
