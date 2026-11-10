package onboard

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"
)

const exchangeRoute = "POST /app/rest/users/current/tokens"

// exchangeServer answers the exchange endpoint with exchangeStatus and /users/current with 200 only for "saved-token" and "exchanged-token".
func exchangeServer(t *testing.T, exchangeStatus int) (string, *onboardingRecorder) {
	t.Helper()
	t.Setenv("TC_INSECURE_SKIP_WARN", "1")
	config.ResetForTest()
	client, rec := newRecordingClient(t, map[string]http.HandlerFunc{
		exchangeRoute: func(w http.ResponseWriter, r *http.Request) {
			if exchangeStatus != http.StatusOK {
				http.Error(w, http.StatusText(exchangeStatus), exchangeStatus)
				return
			}
			writeJSON(w, api.Token{Name: "teamcity-cli", Value: "exchanged-token"})
		},
		"GET /app/rest/users/current": func(w http.ResponseWriter, r *http.Request) {
			switch r.Header.Get("Authorization") {
			case "Bearer saved-token", "Bearer exchanged-token":
				writeJSON(w, api.User{ID: 1, Username: "admin"})
			default:
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
		},
	})
	return client.ServerURL(), rec
}

// saveLogin stores token for serverURL in memory; no User skips the host keyring lookup.
func saveLogin(serverURL, token string) {
	config.Get().Servers[serverURL] = config.ServerConfig{Token: token}
}

func TestAccessTokenExchanges(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	require.NoError(t, config.Init())
	gokeyring.MockInit()
	serverURL, rec := exchangeServer(t, http.StatusOK)
	f, _ := testFactory()

	got, err := accessToken(f, serverURL, "onboarding-token")
	require.NoError(t, err)
	assert.Equal(t, "exchanged-token", got)
	assert.Equal(t, []string{"login-completed"}, rec.endpoints())
	assert.Empty(t, rec.body(t, "login-completed"))
}

func TestAccessTokenFallsBackToSavedLogin(t *testing.T) {
	serverURL, rec := exchangeServer(t, http.StatusUnauthorized)
	saveLogin(serverURL, "saved-token")
	f, out := testFactory()

	got, err := accessToken(f, serverURL, "onboarding-token")
	require.NoError(t, err)
	assert.Equal(t, "saved-token", got)
	assert.Contains(t, out.String(), "using your existing login")
	assert.Equal(t, []string{"login-completed"}, rec.endpoints())
}

func TestAccessTokenIgnoresOlderServer(t *testing.T) {
	run := func(status int) (string, string, error) {
		serverURL, rec := exchangeServer(t, http.StatusUnauthorized)
		rec.respond(status)
		saveLogin(serverURL, "saved-token")
		f, out := testFactory()
		got, err := accessToken(f, serverURL, "onboarding-token")
		assert.Equal(t, []string{"login-completed"}, rec.endpoints())
		return got, strings.ReplaceAll(out.String(), serverURL, "<server>"), err
	}

	wantToken, wantOut, wantErr := run(http.StatusNoContent)
	gotToken, gotOut, gotErr := run(http.StatusNotFound)
	require.NoError(t, gotErr)
	assert.Equal(t, wantErr, gotErr)
	assert.Equal(t, wantToken, gotToken)
	assert.Equal(t, wantOut, gotOut)
}

func TestAccessTokenNoUsableLogin(t *testing.T) {
	for name, saved := range map[string]string{
		"no saved login":      "",
		"saved login expired": "stale-token",
	} {
		t.Run(name, func(t *testing.T) {
			serverURL, rec := exchangeServer(t, http.StatusUnauthorized)
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
			assert.Empty(t, rec.endpoints(), "no login, so nothing reported")
		})
	}
}

func TestAccessTokenWithoutOnboardingToken(t *testing.T) {
	newServer := func(t *testing.T) (string, *onboardingRecorder) {
		t.Helper()
		config.ResetForTest()
		client, rec := newRecordingClient(t, map[string]http.HandlerFunc{
			exchangeRoute: func(w http.ResponseWriter, r *http.Request) {
				t.Error("no exchange expected without --token")
			},
			"GET /app/rest/users/current": func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, api.User{ID: 1, Username: "admin"})
			},
		})
		return client.ServerURL(), rec
	}

	t.Run("uses saved login", func(t *testing.T) {
		serverURL, rec := newServer(t)
		saveLogin(serverURL, "saved-token")
		f, out := testFactory()

		got, err := accessToken(f, serverURL, "")
		require.NoError(t, err)
		assert.Equal(t, "saved-token", got)
		assert.NotContains(t, out.String(), "Couldn't exchange", "no exchange was attempted, so no warning")
		assert.Equal(t, []string{"login-completed"}, rec.endpoints())
	})

	t.Run("not logged in", func(t *testing.T) {
		serverURL, rec := newServer(t)
		f, _ := testFactory()

		_, err := accessToken(f, serverURL, "")
		ve, ok := errors.AsType[*api.ValidationError](err)
		require.True(t, ok, "want *api.ValidationError, got %T: %v", err, err)
		assert.Equal(t, "you're not logged in to "+serverURL, ve.Error())
		assert.Equal(t, "Run teamcity auth login -s "+serverURL+", then teamcity onboard "+serverURL, ve.Suggestion())
		assert.Empty(t, rec.endpoints())
	})
}

func TestAccessTokenOtherFailuresDoNotFallBack(t *testing.T) {
	serverURL, rec := exchangeServer(t, http.StatusBadRequest)
	saveLogin(serverURL, "saved-token")
	f, _ := testFactory()

	_, err := accessToken(f, serverURL, "onboarding-token")
	assert.ErrorContains(t, err, "onboarding token exchange failed")
	assert.Empty(t, rec.endpoints())
}
