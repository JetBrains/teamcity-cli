package onboard

import (
	"errors"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const exchangeRoute = "POST /app/rest/users/current/temporaryTokens/exchange"

// exchangeServer answers the exchange endpoint with exchangeStatus and /users/current with 200 only for "saved-token".
func exchangeServer(t *testing.T, exchangeStatus int) string {
	t.Helper()
	t.Setenv("TC_INSECURE_SKIP_WARN", "1")
	config.ResetForTest()
	return newTestClient(t, map[string]http.HandlerFunc{
		exchangeRoute: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(exchangeStatus), exchangeStatus)
		},
		"GET /app/rest/users/current": func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer saved-token" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			writeJSON(w, api.User{ID: 1, Username: "admin"})
		},
	}).ServerURL()
}

// saveLogin stores token for serverURL in memory; no User skips the host keyring lookup.
func saveLogin(serverURL, token string) {
	config.Get().Servers[serverURL] = config.ServerConfig{Token: token}
}

func TestAccessTokenFallsBackToSavedLogin(t *testing.T) {
	for name, status := range map[string]int{
		"401: server rejects unknown token": http.StatusUnauthorized,
		"404: endpoint missing":             http.StatusNotFound,
	} {
		t.Run(name, func(t *testing.T) {
			serverURL := exchangeServer(t, status)
			saveLogin(serverURL, "saved-token")
			f, out := testFactory()

			got, err := accessToken(f, serverURL, "onboarding-token")
			require.NoError(t, err)
			assert.Equal(t, "saved-token", got)
			assert.Contains(t, out.String(), "using your existing login")
		})
	}
}

func TestAccessTokenNoUsableLogin(t *testing.T) {
	for name, saved := range map[string]string{
		"no saved login":      "",
		"saved login expired": "stale-token",
	} {
		t.Run(name, func(t *testing.T) {
			serverURL := exchangeServer(t, http.StatusUnauthorized)
			if saved != "" {
				saveLogin(serverURL, saved)
			}
			f, _ := testFactory()

			_, err := accessToken(f, serverURL, "onboarding-token")
			ve, ok := errors.AsType[*api.ValidationError](err)
			require.True(t, ok, "want *api.ValidationError, got %T: %v", err, err)
			assert.Contains(t, ve.Error(), "onboarding token is expired or invalid")
			assert.Contains(t, ve.Error(), "not logged in to "+serverURL)
			assert.Contains(t, ve.Suggestion(), "teamcity auth login -s "+serverURL)
		})
	}
}

func TestAccessTokenWithoutOnboardingToken(t *testing.T) {
	newServer := func(t *testing.T) string {
		t.Helper()
		config.ResetForTest()
		return newTestClient(t, map[string]http.HandlerFunc{
			exchangeRoute: func(w http.ResponseWriter, r *http.Request) {
				t.Error("no exchange expected without --token")
			},
			"GET /app/rest/users/current": func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, api.User{ID: 1, Username: "admin"})
			},
		}).ServerURL()
	}

	t.Run("uses saved login", func(t *testing.T) {
		serverURL := newServer(t)
		saveLogin(serverURL, "saved-token")
		f, out := testFactory()

		got, err := accessToken(f, serverURL, "")
		require.NoError(t, err)
		assert.Equal(t, "saved-token", got)
		assert.NotContains(t, out.String(), "Couldn't exchange", "no exchange was attempted, so no warning")
	})

	t.Run("not logged in", func(t *testing.T) {
		serverURL := newServer(t)
		f, _ := testFactory()

		_, err := accessToken(f, serverURL, "")
		ve, ok := errors.AsType[*api.ValidationError](err)
		require.True(t, ok, "want *api.ValidationError, got %T: %v", err, err)
		assert.Equal(t, "you're not logged in to "+serverURL, ve.Error())
		assert.Equal(t, "Run teamcity auth login -s "+serverURL+", then teamcity onboard "+serverURL, ve.Suggestion())
	})
}

func TestAccessTokenOtherFailuresDoNotFallBack(t *testing.T) {
	serverURL := exchangeServer(t, http.StatusForbidden)
	saveLogin(serverURL, "saved-token")
	f, _ := testFactory()

	_, err := accessToken(f, serverURL, "onboarding-token")
	assert.ErrorContains(t, err, "not a valid onboarding token")
}
