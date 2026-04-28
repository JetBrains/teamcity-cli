package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyStandardHeadersOnEveryEntryPoint asserts that for every code path that builds an HTTP
// request — typed, raw, reboot, download, probe, and PKCE — the client sends User-Agent,
// X-TeamCity-Client, and any caller-configured extra headers. This is the single chokepoint test.
func TestApplyStandardHeadersOnEveryEntryPoint(T *testing.T) {
	T.Parallel()

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
		{"ExchangeCodeForToken", func(c *Client) {
			_, _ = c.ExchangeCodeForToken(T.Context(), "code", "verifier", "http://localhost/cb")
		}},
	}
	for _, tc := range cases {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got http.Header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Clone()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			t.Cleanup(server.Close)

			c := NewClient(server.URL, "tok", WithExtraHeaders(map[string]string{
				"CF-Access-Client-Id":     "abc.id",
				"CF-Access-Client-Secret": "shh",
			}))
			tc.run(c)

			assert.Contains(t, got.Get("User-Agent"), "teamcity-cli/")
			assert.Contains(t, got.Get("X-TeamCity-Client"), "teamcity-cli/")
			assert.Equal(t, "abc.id", got.Get("Cf-Access-Client-Id"))
			assert.Equal(t, "shh", got.Get("Cf-Access-Client-Secret"))
		})
	}
}

func TestEnvHeaders(T *testing.T) {
	// No T.Parallel: subtests use t.Setenv, which forbids any parallel ancestor.

	T.Run("nil when no matching env vars", func(t *testing.T) {
		t.Setenv("UNRELATED_ENV", "x")
		assert.Nil(t, EnvHeaders())
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
		_, present := EnvHeaders()["X-Empty"]
		assert.False(t, present)
	})

	T.Run("drops values with CR or LF (header-injection guard)", func(t *testing.T) {
		// NUL bytes can't be set via os.Setenv on most platforms; the NUL guard is
		// exercised via WithExtraHeaders in TestWithExtraHeaders_DropsCRLFAtConstruction.
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
