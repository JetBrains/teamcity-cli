package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPkcePermissions(t *testing.T) {
	t.Parallel()
	defaults := DefaultScopes()
	assert.Len(t, defaults, 29, "additional permissions must not expand the default grant")
	for _, scope := range defaults {
		assert.Contains(t, KnownPermissions, scope)
	}
	for _, scope := range []string{"EDIT_VERSIONED_SETTINGS", "CHANGE_SERVER_SETTINGS"} {
		assert.Contains(t, KnownPermissions, scope)
		assert.NotContains(t, defaults, scope)
	}
	assert.NotContains(t, KnownPermissions, "CHANGE_OWN_PROFILE", "TeamCity prohibits this PKCE scope")
	assert.Equal(t, "EDIT_VERSIONED_SETTINGS", PermissionEnum("Enable / disable versioned settings"))
}
