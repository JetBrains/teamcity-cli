package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAuthorizeURL(T *testing.T) {
	T.Parallel()

	T.Run("includes all required parameters", func(t *testing.T) {
		t.Parallel()

		authURL := BuildAuthorizeURL(
			"https://teamcity.example.com",
			"http://localhost:19000/callback",
			"challenge123",
			"state456",
			[]string{"RUN_BUILD", "VIEW_PROJECT"},
		)

		parsed, err := url.Parse(authURL)
		require.NoError(t, err)

		assert.Equal(t, "https", parsed.Scheme)
		assert.Equal(t, "teamcity.example.com", parsed.Host)
		assert.Equal(t, "/pkce/authorize.html", parsed.Path)

		query := parsed.Query()
		assert.Equal(t, "teamcity-cli", query.Get("client_id"))
		assert.Equal(t, "code", query.Get("response_type"))
		assert.Equal(t, "http://localhost:19000/callback", query.Get("redirect_uri"))
		assert.Equal(t, "challenge123", query.Get("code_challenge"))
		assert.Equal(t, "S256", query.Get("code_challenge_method"))
		assert.Equal(t, "state456", query.Get("state"))
		assert.Equal(t, "RUN_BUILD VIEW_PROJECT", query.Get("scope"))
	})

	T.Run("strips trailing slash from server URL", func(t *testing.T) {
		t.Parallel()

		authURL := BuildAuthorizeURL(
			"https://teamcity.example.com/",
			"http://localhost:19000/callback",
			"challenge",
			"state",
			[]string{"RUN_BUILD"},
		)

		assert.True(t, strings.HasPrefix(authURL, "https://teamcity.example.com/pkce/"))
		assert.NotContains(t, authURL, "//pkce")
	})
}

func TestIsPkceEnabled(T *testing.T) {
	T.Parallel()

	T.Run("returns true when server responds 200", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/pkce/is_enabled.html", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		enabled, err := NewGuestClient(server.URL).IsPkceEnabled(T.Context())
		assert.NoError(t, err)
		assert.True(t, enabled)
	})

	T.Run("returns false when server responds 404", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(server.Close)

		enabled, err := NewGuestClient(server.URL).IsPkceEnabled(T.Context())
		assert.NoError(t, err)
		assert.False(t, enabled)
	})

	T.Run("returns error on network failure", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(T.Context(), 100*time.Millisecond)
		defer cancel()
		enabled, err := NewGuestClient("http://localhost:1").IsPkceEnabled(ctx)
		assert.Error(t, err)
		assert.False(t, enabled)
	})
}

func TestExchangeCodeForToken(T *testing.T) {
	T.Parallel()

	T.Run("exchanges code for token successfully", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/pkce/token.html", r.URL.Path)
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			assert.Equal(t, "teamcity-cli", r.Form.Get("client_id"))
			assert.Equal(t, "testcode", r.Form.Get("code"))
			assert.Equal(t, "testverifier", r.Form.Get("code_verifier"))
			assert.Equal(t, "http://localhost:19000/callback", r.Form.Get("redirect_uri"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"token123","token_type":"Bearer","valid_until":"2026-03-03T11:25:51.572Z"}`))
		}))
		t.Cleanup(server.Close)

		token, err := NewGuestClient(server.URL).ExchangeCodeForToken(T.Context(), "testcode", "testverifier", "http://localhost:19000/callback")
		require.NoError(t, err)
		assert.Equal(t, "token123", token.AccessToken)
		assert.Equal(t, "Bearer", token.TokenType)
		assert.Equal(t, "2026-03-03T11:25:51.572Z", token.ValidUntil)
	})

	T.Run("returns error on invalid code", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("Invalid authorization code"))
		}))
		t.Cleanup(server.Close)

		_, err := NewGuestClient(server.URL).ExchangeCodeForToken(T.Context(), "invalidcode", "verifier", "http://localhost:19000/callback")
		assert.Error(t, err)
	})
}
