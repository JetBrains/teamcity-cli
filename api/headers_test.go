package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStandardHeadersOnEveryEntryPoint asserts that User-Agent and X-TeamCity-Client are
// set on requests from every code path: typed (doRequestFull), raw (doRawRequest),
// reboot (RebootAgent), download (DownloadArtifactTo), probe, and PKCE.
func TestStandardHeadersOnEveryEntryPoint(T *testing.T) {
	T.Parallel()

	check := func(t *testing.T, h http.Header) {
		t.Helper()
		ua := h.Get("User-Agent")
		tc := h.Get("X-TeamCity-Client")
		assert.True(t, strings.HasPrefix(ua, "teamcity-cli/"), "User-Agent should be teamcity-cli/...; got %q", ua)
		assert.True(t, strings.HasPrefix(tc, "teamcity-cli/"), "X-TeamCity-Client should be teamcity-cli/...; got %q", tc)
	}

	T.Run("typed GET via doRequestFull", func(t *testing.T) {
		t.Parallel()
		var got http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{}`))
		}))
		t.Cleanup(server.Close)

		_, _ = NewClient(server.URL, "tok").GetServer()
		check(t, got)
	})

	T.Run("raw via doRawRequest", func(t *testing.T) {
		t.Parallel()
		var got http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		_, _ = NewClient(server.URL, "tok").RawRequest(T.Context(), "GET", "/x", nil, nil)
		check(t, got)
	})

	T.Run("RebootAgent", func(t *testing.T) {
		t.Parallel()
		var got http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		_ = NewClient(server.URL, "tok").RebootAgent(T.Context(), 1, false)
		check(t, got)
	})

	T.Run("DownloadArtifactTo", func(t *testing.T) {
		t.Parallel()
		var got http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		var buf bytes.Buffer
		_, _ = NewClient(server.URL, "tok").DownloadArtifactTo(T.Context(), "1", "x.txt", &buf)
		check(t, got)
	})

	T.Run("Probe", func(t *testing.T) {
		t.Parallel()
		var got http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		_ = NewGuestClient(server.URL).Probe(T.Context())
		check(t, got)
	})

	T.Run("IsPkceEnabled", func(t *testing.T) {
		t.Parallel()
		var got http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		_, _ = NewGuestClient(server.URL).IsPkceEnabled(T.Context())
		check(t, got)
	})

	T.Run("ExchangeCodeForToken", func(t *testing.T) {
		t.Parallel()
		var got http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"t"}`))
		}))
		t.Cleanup(server.Close)

		_, _ = NewGuestClient(server.URL).ExchangeCodeForToken(T.Context(), "code", "verifier", "http://localhost/cb")
		check(t, got)

		// And content-type for the form payload is preserved.
		assert.Equal(T, "application/x-www-form-urlencoded", got.Get("Content-Type"))
	})
}
