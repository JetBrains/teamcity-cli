package cmdutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotAuthenticatedErrorSuggestion(t *testing.T) {
	t.Parallel()

	err := NotAuthenticatedError("", nil)

	assert.Equal(t, "Not authenticated", err.Message)
	assert.Contains(t, err.Suggestion, "set both TEAMCITY_URL and TEAMCITY_TOKEN")
	assert.Contains(t, err.Suggestion, "unset TEAMCITY_URL to use stored auth")
	assert.Contains(t, err.Suggestion, "teamcity auth login --insecure-storage")
}

func TestNotAuthenticatedErrorIncludesKeyringError(t *testing.T) {
	t.Parallel()

	err := NotAuthenticatedError("", errors.New("keyring unavailable"))

	assert.Equal(t, "Not authenticated (could not access system keyring: keyring unavailable)", err.Message)
}
