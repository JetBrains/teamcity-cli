package auth_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWebLoginServer(t *testing.T, pkceEnabled bool) *cmdtest.TestServer {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ts := cmdtest.NewTestServer(t)
	t.Setenv("TEAMCITY_URL", "")
	t.Setenv("TEAMCITY_TOKEN", "")
	ts.Handle("POST "+api.PkceIsEnabledPath, func(w http.ResponseWriter, r *http.Request) {
		if !pkceEnabled {
			http.NotFound(w, r)
		}
	})
	ts.Handle("POST "+api.PkceTokenPath, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		if r.PostForm.Get("code") != "the-code" || r.PostForm.Get("code_verifier") == "" {
			http.Error(w, "bad grant", http.StatusBadRequest)
			return
		}
		cmdtest.JSON(w, api.TokenResponse{AccessToken: "web-token", TokenType: "Bearer", ValidUntil: "2027-01-01T00:00:00Z"})
	})
	ts.Handle("GET /app/rest/users/current", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer web-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		cmdtest.JSON(w, api.User{ID: 7, Username: "jdoe", Name: "Jane Doe"})
	})
	ts.Handle("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Server{VersionMajor: 2026, VersionMinor: 1})
	})
	return ts
}

// approveInBrowser stands in for the browser: it follows the authorize URL straight to the local callback.
func approveInBrowser(t *testing.T, openErr error) {
	t.Helper()
	orig := cmdutil.OpenInBrowser
	t.Cleanup(func() { cmdutil.OpenInBrowser = orig })
	cmdutil.OpenInBrowser = func(authURL string) error {
		u, err := url.Parse(authURL)
		require.NoError(t, err)
		q := u.Query()
		resp, err := http.Get(q.Get("redirect_uri") + "?" + url.Values{"code": {"the-code"}, "state": {q.Get("state")}}.Encode())
		require.NoError(t, err)
		_ = resp.Body.Close()
		return openErr
	}
}

func TestAuthLoginWebJSON(T *testing.T) {
	for _, tc := range []struct {
		name    string
		openErr error
	}{
		{"browser opened", nil},
		{"browser unavailable", errors.New("no display")},
	} {
		T.Run(tc.name, func(t *testing.T) {
			ts := newWebLoginServer(t, true)
			approveInBrowser(t, tc.openErr)

			got := cmdtest.CaptureOutput(t, ts.Factory, "auth", "login", "--server", ts.URL, "--web", "--json", "--insecure-storage")

			lines := strings.Split(strings.TrimSpace(got), "\n")
			require.Len(t, lines, 2, got)
			var authorize, login map[string]any
			require.NoError(t, json.Unmarshal([]byte(lines[0]), &authorize))
			require.NoError(t, json.Unmarshal([]byte(lines[1]), &login))

			assert.Equal(t, "authorize", authorize["event"])
			assert.True(t, strings.HasPrefix(authorize["url"].(string), ts.URL+api.PkceAuthorizePath+"?"))
			assert.Equal(t, tc.openErr == nil, authorize["browser_opened"])

			assert.Equal(t, "login", login["event"])
			assert.Equal(t, ts.URL, login["server"])
			assert.Equal(t, "token", login["auth_method"])
			assert.Equal(t, "config", login["token_storage"])
			assert.Equal(t, "2027-01-01T00:00:00Z", login["token_expiry"])
			assert.Equal(t, map[string]any{"id": float64(7), "username": "jdoe", "name": "Jane Doe"}, login["user"])

			assert.Equal(t, "web-token", config.Get().Servers[ts.URL].Token)
		})
	}
}

func TestAuthLoginWebPrintsURL(T *testing.T) {
	ts := newWebLoginServer(T, true)
	var opened string
	approveInBrowser(T, nil)
	approve := cmdutil.OpenInBrowser
	cmdutil.OpenInBrowser = func(u string) error { opened = u; return approve(u) }

	got := cmdtest.CaptureOutput(T, ts.Factory, "auth", "login", "--server", ts.URL, "--web", "--insecure-storage")

	assert.Contains(T, got, "\n"+opened+"\n", "authorization URL must be on its own line")
	assert.Contains(T, got, "Logged in to")
}

func TestAuthLoginWebFailsWithoutPkce(T *testing.T) {
	ts := newWebLoginServer(T, false)
	err := cmdtest.CaptureErr(T, ts.Factory, "auth", "login", "--server", ts.URL, "--web")
	assert.Contains(T, err.Error(), "browser sign-in is not available")
}

func TestAuthLoginWebRequiresServer(T *testing.T) {
	ts := newWebLoginServer(T, true)
	ts.Factory.NoInput = true
	err := cmdtest.CaptureErr(T, ts.Factory, "auth", "login", "--web")
	assert.Contains(T, err.Error(), "--server is required")
}

func TestAuthLoginWebExclusiveFlags(T *testing.T) {
	ts := newWebLoginServer(T, true)
	for _, flag := range [][]string{{"--token", "x"}, {"--guest"}, {"--no-browser"}} {
		args := append([]string{"auth", "login", "--server", ts.URL, "--web"}, flag...)
		err := cmdtest.CaptureErr(T, ts.Factory, args...)
		assert.Contains(T, err.Error(), "none of the others can be")
	}
}
