package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnboardingTokenPermissionsCannotCreateTokens(t *testing.T) {
	for _, p := range []string{"CREATE_OWN_ACCESS_TOKEN", "CHANGE_OWN_PROFILE"} {
		assert.NotContains(t, onboardingTokenPermissions, p)
	}
}

func TestRestrictToken(t *testing.T) {
	exchanged := &api.Token{Name: "teamcity-cli-x-exchange", Value: "unrestricted"}
	printer := &output.Printer{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}

	t.Run("creates a restricted token and deletes the exchange token", func(t *testing.T) {
		var deleted string
		client := restrictServer(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				var req api.Token
				require.NoError(t, json.Unmarshal(body, &req))
				assert.Equal(t, "teamcity-cli-x", req.Name)
				assert.NotEmpty(t, req.ExpirationTime)
				require.NotNil(t, req.PermissionRestrictions)
				ids := make([]string, 0, len(req.PermissionRestrictions.PermissionRestriction))
				for _, pr := range req.PermissionRestrictions.PermissionRestriction {
					assert.True(t, pr.IsGlobalScope)
					ids = append(ids, pr.Permission.ID)
				}
				assert.Equal(t, onboardingTokenPermissions, ids)
				assert.False(t, slices.Contains(ids, "CREATE_OWN_ACCESS_TOKEN"))
				_ = json.NewEncoder(w).Encode(api.Token{Name: req.Name, Value: "restricted"})
			case http.MethodDelete:
				deleted = r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			}
		})

		got := restrictToken(printer, client, exchanged, "teamcity-cli-x", time.Now().Add(time.Hour))
		assert.Equal(t, "restricted", got)
		assert.Equal(t, "/app/rest/users/current/tokens/teamcity-cli-x-exchange", deleted)
	})

	t.Run("keeps the exchange token when the server rejects the restriction", func(t *testing.T) {
		deleteCalled := false
		client := restrictServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				deleteCalled = true
			}
			w.WriteHeader(http.StatusForbidden)
		})

		got := restrictToken(printer, client, exchanged, "teamcity-cli-x", time.Now().Add(time.Hour))
		assert.Equal(t, "unrestricted", got)
		assert.False(t, deleteCalled, "the token still in use must not be deleted")
	})
}

func restrictServer(t *testing.T, handler http.HandlerFunc) *api.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return api.NewClient(server.URL, "unrestricted")
}
