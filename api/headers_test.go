package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// init guards against accidentally inheriting TEAMCITY_HEADER_* env vars from CI or a
// developer shell. Tests that need them set must use t.Setenv per-test; a stray export
// would silently change request payloads in unrelated tests.
func init() {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, EnvHeaderPrefix) {
			panic(fmt.Sprintf("api tests must run with no %s* env set; found: %s", EnvHeaderPrefix, e))
		}
	}
}

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

func TestEnvHeaders(T *testing.T) {
	// No T.Parallel: subtests use t.Setenv, which forbids any parallel ancestor.

	T.Run("nil when no matching env vars", func(t *testing.T) {
		t.Setenv("UNRELATED_ENV", "x")
		got := EnvHeaders()
		assert.Nil(t, got)
	})

	T.Run("translates underscores to hyphens and canonicalizes case", func(t *testing.T) {
		t.Setenv("TEAMCITY_HEADER_CF_ACCESS_CLIENT_ID", "abc.id")
		t.Setenv("TEAMCITY_HEADER_X_FOO", "bar")

		got := EnvHeaders()
		assert.Equal(t, "abc.id", got["Cf-Access-Client-Id"])
		assert.Equal(t, "bar", got["X-Foo"])
	})

	T.Run("drops empty values", func(t *testing.T) {
		t.Setenv("TEAMCITY_HEADER_X_EMPTY", "")
		got := EnvHeaders()
		_, present := got["X-Empty"]
		assert.False(t, present, "empty value should not produce a header")
	})

	T.Run("drops values with CR or LF (header-injection guard)", func(t *testing.T) {
		// NUL bytes can't be set via os.Setenv on most platforms; the guard for them
		// is exercised in TestWithExtraHeaders_DropsCRLFAtConstruction below.
		t.Setenv("TEAMCITY_HEADER_X_CR", "value\rinjected")
		t.Setenv("TEAMCITY_HEADER_X_LF", "value\ninjected")
		t.Setenv("TEAMCITY_HEADER_X_GOOD", "ok")

		got := EnvHeaders()
		_, hasCR := got["X-Cr"]
		_, hasLF := got["X-Lf"]
		assert.False(t, hasCR)
		assert.False(t, hasLF)
		assert.Equal(t, "ok", got["X-Good"])
	})
}

func TestWithExtraHeaders_AppliedOnEveryEntryPoint(T *testing.T) {
	T.Parallel()

	probe := func(t *testing.T, run func(c *Client)) http.Header {
		t.Helper()
		var got http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		t.Cleanup(server.Close)

		c := NewClient(server.URL, "tok", WithExtraHeaders(map[string]string{
			"CF-Access-Client-Id":     "abc.id",
			"CF-Access-Client-Secret": "shh",
		}))
		run(c)
		return got
	}

	cases := []struct {
		name string
		run  func(c *Client)
	}{
		{"typed GET", func(c *Client) { _, _ = c.GetServer() }},
		{"RawRequest", func(c *Client) { _, _ = c.RawRequest(T.Context(), "GET", "/x", nil, nil) }},
		{"RebootAgent", func(c *Client) { _ = c.RebootAgent(T.Context(), 1, false) }},
		{"DownloadArtifactTo", func(c *Client) {
			var b bytes.Buffer
			_, _ = c.DownloadArtifactTo(T.Context(), "1", "x.txt", &b)
		}},
		{"Probe", func(c *Client) { _ = c.Probe(T.Context()) }},
		{"IsPkceEnabled", func(c *Client) { _, _ = c.IsPkceEnabled(T.Context()) }},
	}
	for _, tc := range cases {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := probe(t, tc.run)
			assert.Equal(t, "abc.id", got.Get("Cf-Access-Client-Id"))
			assert.Equal(t, "shh", got.Get("Cf-Access-Client-Secret"))
		})
	}
}

func TestWithExtraHeaders_RawRequestPerRequestOverrides(T *testing.T) {
	T.Parallel()

	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	T.Cleanup(server.Close)

	c := NewClient(server.URL, "tok", WithExtraHeaders(map[string]string{
		"X-Common": "from-extras",
	}))
	_, err := c.RawRequest(T.Context(), "GET", "/x", nil, map[string]string{
		"X-Common": "from-call-site",
	})
	require.NoError(T, err)

	assert.Equal(T, "from-call-site", got.Get("X-Common"), "per-request headers must override extras")
}

func TestWithExtraHeaders_DropsCRLFAtConstruction(T *testing.T) {
	T.Parallel()

	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	T.Cleanup(server.Close)

	c := NewClient(server.URL, "tok", WithExtraHeaders(map[string]string{
		"X-Bad":  "value\r\nInjected: yes",
		"X-Nul":  "value\x00injected",
		"X-Good": "ok",
		"":       "no-name",
	}))
	_, _ = c.GetServer()
	assert.Empty(T, got.Get("X-Bad"))
	assert.Empty(T, got.Get("X-Nul"))
	assert.Empty(T, got.Get("Injected"))
	assert.Equal(T, "ok", got.Get("X-Good"))
}

func TestWithExtraHeaders_RedactedInDebugLog(T *testing.T) {
	T.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	T.Cleanup(server.Close)

	var debug bytes.Buffer
	c := NewClient(server.URL, "tok",
		WithExtraHeaders(map[string]string{"X-Secret-Header": "verysecret"}),
		WithDebugFunc(func(format string, args ...any) {
			fmt.Fprintf(&debug, format+"\n", args...)
		}),
	)
	_, _ = c.GetServer()

	out := debug.String()
	assert.Contains(T, out, "X-Secret-Header: [REDACTED]")
	assert.NotContains(T, out, "verysecret", "extra-header values must never appear in debug output")
}
