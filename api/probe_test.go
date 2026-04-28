package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientProbe(T *testing.T) {
	T.Parallel()

	T.Run("returns nil on 200", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/app/rest/server/version", r.URL.Path)
			assert.Empty(t, r.Header.Get("Authorization"), "probe must not send credentials")
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		client := NewGuestClient(server.URL)
		assert.NoError(t, client.Probe(T.Context()))
	})

	T.Run("returns nil on 401/403 (auth-protected but reachable)", func(t *testing.T) {
		t.Parallel()

		for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			t.Cleanup(server.Close)

			client := NewClient(server.URL, "ignored-token")
			assert.NoError(t, client.Probe(T.Context()))
		}
	})

	T.Run("returns ErrLoginGatewayDetected when API returns HTML", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body>Login</body></html>"))
		}))
		t.Cleanup(server.Close)

		client := NewGuestClient(server.URL)
		assert.ErrorIs(t, client.Probe(T.Context()), ErrLoginGatewayDetected)
	})

	T.Run("returns error on 404", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(server.Close)

		client := NewGuestClient(server.URL)
		err := client.Probe(T.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a TeamCity server")
	})

	T.Run("returns error on unexpected status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)

		client := NewGuestClient(server.URL)
		err := client.Probe(T.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP 500")
	})

	T.Run("preserves transport (defaultTransport not dropped)", func(t *testing.T) {
		t.Parallel()

		client := NewGuestClient("https://example.com")
		assert.NotNil(t, client.HTTPClient.Transport, "probe path must keep the configured transport")
	})
}
