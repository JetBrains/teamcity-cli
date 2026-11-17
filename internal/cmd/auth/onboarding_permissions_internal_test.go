package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOnboardingTokenPermissionsCannotCreateTokens(t *testing.T) {
	// The server rejects a restriction with either, and with them the token could create more tokens.
	for _, p := range []string{"CREATE_OWN_ACCESS_TOKEN", "CHANGE_OWN_PROFILE"} {
		assert.NotContains(t, onboardingTokenPermissions, p)
	}
}
