//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerBootstrapAuthentication(t *testing.T) {
	if testEnvRef == nil || !testEnvRef.ownsContainers {
		t.Skip("only checks authentication on the disposable test server")
	}
	response, err := client.RawRequest(t.Context(), http.MethodGet, "/app/rest/server/authSettings", nil, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	var settings struct {
		PerProjectPermissions bool `json:"perProjectPermissions"`
		Modules               struct {
			Module []struct {
				Name string `json:"name"`
			} `json:"module"`
		} `json:"modules"`
	}
	require.NoError(t, json.Unmarshal(response.Body, &settings))
	assert.True(t, settings.PerProjectPermissions)
	require.NotEmpty(t, settings.Modules.Module, "bootstrap must preserve authentication modules")
	user, err := client.GetCurrentUser()
	require.NoError(t, err)
	assert.Equal(t, "admin", user.Username)
}
