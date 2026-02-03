package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCodeVerifier(T *testing.T) {
	T.Parallel()

	T.Run("length is at least 43 characters", func(t *testing.T) {
		t.Parallel()

		verifier, err := GenerateCodeVerifier()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(verifier), 43, "RFC 7636 requires minimum 43 characters")
	})

	T.Run("contains only URL-safe characters", func(t *testing.T) {
		t.Parallel()

		verifier, err := GenerateCodeVerifier()
		require.NoError(t, err)
		// base64url alphabet: A-Z, a-z, 0-9, -, _
		urlSafePattern := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
		assert.True(t, urlSafePattern.MatchString(verifier), "verifier should only contain URL-safe base64 characters")
	})

	T.Run("no padding characters", func(t *testing.T) {
		t.Parallel()

		verifier, err := GenerateCodeVerifier()
		require.NoError(t, err)
		assert.NotContains(t, verifier, "=", "verifier should not contain padding")
	})

	T.Run("generates unique values", func(t *testing.T) {
		t.Parallel()

		v1, err := GenerateCodeVerifier()
		require.NoError(t, err)
		v2, err := GenerateCodeVerifier()
		require.NoError(t, err)
		assert.NotEqual(t, v1, v2, "verifiers should be unique")
	})
}

func TestGenerateCodeChallenge(T *testing.T) {
	T.Parallel()

	T.Run("produces valid base64url encoded SHA256 hash", func(t *testing.T) {
		t.Parallel()

		verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge := GenerateCodeChallenge(verifier)

		// Should be base64url encoded (no padding, URL-safe chars)
		urlSafePattern := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
		assert.True(t, urlSafePattern.MatchString(challenge), "challenge should be base64url encoded")
	})

	T.Run("no padding characters", func(t *testing.T) {
		t.Parallel()

		verifier, err := GenerateCodeVerifier()
		require.NoError(t, err)
		challenge := GenerateCodeChallenge(verifier)
		assert.NotContains(t, challenge, "=", "challenge should not contain padding")
	})

	T.Run("same verifier produces same challenge", func(t *testing.T) {
		t.Parallel()

		verifier := "test-verifier-12345"
		c1 := GenerateCodeChallenge(verifier)
		c2 := GenerateCodeChallenge(verifier)
		assert.Equal(t, c1, c2, "same verifier should produce same challenge")
	})

	T.Run("different verifiers produce different challenges", func(t *testing.T) {
		t.Parallel()

		c1 := GenerateCodeChallenge("verifier1")
		c2 := GenerateCodeChallenge("verifier2")
		assert.NotEqual(t, c1, c2, "different verifiers should produce different challenges")
	})

	T.Run("challenge can be decoded as valid base64url", func(t *testing.T) {
		t.Parallel()

		verifier, err := GenerateCodeVerifier()
		require.NoError(t, err)
		challenge := GenerateCodeChallenge(verifier)

		decoded, err := base64.RawURLEncoding.DecodeString(challenge)
		require.NoError(t, err, "challenge should be valid base64url")
		assert.Len(t, decoded, 32, "SHA256 produces 32 bytes")
	})
}

func TestGenerateState(T *testing.T) {
	T.Parallel()

	T.Run("generates non-empty state", func(t *testing.T) {
		t.Parallel()

		state, err := GenerateState()
		require.NoError(t, err)
		assert.NotEmpty(t, state)
	})

	T.Run("contains only URL-safe characters", func(t *testing.T) {
		t.Parallel()

		state, err := GenerateState()
		require.NoError(t, err)
		urlSafePattern := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
		assert.True(t, urlSafePattern.MatchString(state), "state should only contain URL-safe base64 characters")
	})

	T.Run("generates unique values", func(t *testing.T) {
		t.Parallel()

		s1, err := GenerateState()
		require.NoError(t, err)
		s2, err := GenerateState()
		require.NoError(t, err)
		assert.NotEqual(t, s1, s2, "states should be unique")
	})
}

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
		assert.Equal(t, "code", query.Get("response_type"))
		assert.Equal(t, "http://localhost:19000/callback", query.Get("redirect_uri"))
		assert.Equal(t, "challenge123", query.Get("code_challenge"))
		assert.Equal(t, "S256", query.Get("code_challenge_method"))
		assert.Equal(t, "state456", query.Get("state"))
		assert.Equal(t, "RUN_BUILD VIEW_PROJECT", query.Get("scope"))
	})

	T.Run("handles single scope", func(t *testing.T) {
		t.Parallel()

		authURL := BuildAuthorizeURL(
			"https://teamcity.example.com",
			"http://localhost:19000/callback",
			"challenge",
			"state",
			[]string{"RUN_BUILD"},
		)

		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		assert.Equal(t, "RUN_BUILD", parsed.Query().Get("scope"))
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

func TestFindAvailableListener(T *testing.T) {
	T.Parallel()

	T.Run("returns port in valid range", func(t *testing.T) {
		t.Parallel()

		listener, port, err := FindAvailableListener()
		require.NoError(t, err)
		defer listener.Close()

		assert.GreaterOrEqual(t, port, CallbackPortMin)
		assert.LessOrEqual(t, port, CallbackPortMax)
	})

	T.Run("returned listener is usable", func(t *testing.T) {
		t.Parallel()

		listener, port, err := FindAvailableListener()
		require.NoError(t, err)
		defer listener.Close()

		// Listener should be bound to the reported port
		addr := listener.Addr().(*net.TCPAddr)
		assert.Equal(t, port, addr.Port)
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

		enabled, err := IsPkceEnabled(context.Background(), server.URL)
		assert.NoError(t, err)
		assert.True(t, enabled)
	})

	T.Run("returns false when server responds 404", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(server.Close)

		enabled, err := IsPkceEnabled(context.Background(), server.URL)
		assert.NoError(t, err)
		assert.False(t, enabled)
	})

	T.Run("returns error on network failure", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		enabled, err := IsPkceEnabled(ctx, "http://localhost:1")
		assert.Error(t, err)
		assert.False(t, enabled)
	})
}

func TestCallbackServer(T *testing.T) {
	T.Run("receives authorization code from callback", func(t *testing.T) {
		listener, port, err := FindAvailableListener()
		require.NoError(t, err)

		server := NewCallbackServer(listener, port)
		server.Start()
		defer server.Shutdown()

		go func() {
			time.Sleep(50 * time.Millisecond)
			resp, _ := http.Get(fmt.Sprintf("http://localhost:%d/callback?code=testcode123&state=teststate456", port))
			if resp != nil {
				resp.Body.Close()
			}
		}()

		select {
		case result := <-server.ResultChan:
			assert.Equal(t, "testcode123", result.Code)
			assert.Equal(t, "teststate456", result.State)
			assert.Empty(t, result.Error)
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for callback")
		}
	})

	T.Run("handles error in callback", func(t *testing.T) {
		listener, port, err := FindAvailableListener()
		require.NoError(t, err)

		server := NewCallbackServer(listener, port)
		server.Start()
		defer server.Shutdown()

		go func() {
			time.Sleep(50 * time.Millisecond)
			resp, _ := http.Get(fmt.Sprintf("http://localhost:%d/callback?error=access_denied", port))
			if resp != nil {
				resp.Body.Close()
			}
		}()

		select {
		case result := <-server.ResultChan:
			assert.Empty(t, result.Code)
			assert.Equal(t, "access_denied", result.Error)
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for callback")
		}
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
			assert.Equal(t, "testcode", r.Form.Get("code"))
			assert.Equal(t, "testverifier", r.Form.Get("code_verifier"))
			assert.Equal(t, "http://localhost:19000/callback", r.Form.Get("redirect_uri"))
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"token123","token_type":"Bearer","valid_until":"2026-03-03T11:25:51.572Z"}`))
		}))
		t.Cleanup(server.Close)

		token, err := ExchangeCodeForToken(context.Background(), server.URL, "testcode", "testverifier", "http://localhost:19000/callback")
		require.NoError(t, err)
		assert.Equal(t, "token123", token.AccessToken)
		assert.Equal(t, "Bearer", token.TokenType)
		assert.Equal(t, "2026-03-03T11:25:51.572Z", token.ValidUntil)
	})

	T.Run("returns error on invalid code", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Invalid authorization code"))
		}))
		t.Cleanup(server.Close)

		_, err := ExchangeCodeForToken(context.Background(), server.URL, "invalidcode", "verifier", "http://localhost:19000/callback")
		assert.Error(t, err)
	})
}

func TestDefaultScopes(T *testing.T) {
	T.Parallel()

	T.Run("returns copy of available scopes", func(t *testing.T) {
		t.Parallel()

		scopes := DefaultScopes()
		assert.Equal(t, AvailableScopes, scopes)
		scopes[0] = "MODIFIED"
		assert.NotEqual(t, AvailableScopes[0], "MODIFIED")
	})
}
