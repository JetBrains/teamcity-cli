package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"0.5.0", "0.6.0", true},
		{"0.6.0", "0.6.0", false},
		{"0.7.0", "0.6.0", false},
		{"0.5.0", "1.0.0", true},
		{"1.0.0", "0.9.0", false},
		{"0.5.0", "0.5.1", true},
		{"0.5.1", "0.5.0", false},
		{"v0.5.0", "v0.6.0", true},
		{"0.5.0-rc1", "0.5.0", false},
		{"dev", "0.5.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.current+"_vs_"+tt.latest, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNewer(tt.current, tt.latest))
		})
	}
}

func TestParseSemver(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in                  string
		major, minor, patch int
	}{
		{"1.2.3", 1, 2, 3},
		{"v1.2.3", 1, 2, 3},
		{"v0.10.0", 0, 10, 0},
		{"1.0.0-beta.1", 1, 0, 0}, // pre-release stripped
		{"v2", 2, 0, 0},           // missing parts → zero
		{"", 0, 0, 0},
		{"dev", 0, 0, 0}, // non-numeric → zero
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			major, minor, pat := parseSemver(tc.in)
			assert.Equal(t, tc.major, major, "major")
			assert.Equal(t, tc.minor, minor, "minor")
			assert.Equal(t, tc.patch, pat, "patch")
		})
	}
}

func TestLatestReleaseFromAPI(t *testing.T) {
	const defaultURL = "https://api.github.com/repos/JetBrains/teamcity-cli/releases/latest"

	t.Run("happy path", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
			assert.Equal(t, "teamcity-cli", r.Header.Get("User-Agent"))
			_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","html_url":"https://github.com/x/y/releases/tag/v1.2.3"}`))
		}))
		defer srv.Close()

		t.Cleanup(func() { latestReleaseAPIURL = defaultURL })
		latestReleaseAPIURL = srv.URL

		got, err := latestReleaseFromAPI(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "1.2.3", got.Version) // "v" prefix stripped
		assert.Equal(t, "https://github.com/x/y/releases/tag/v1.2.3", got.URL)
	})

	t.Run("non-200 → error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden) // simulate rate limit
		}))
		defer srv.Close()
		t.Cleanup(func() { latestReleaseAPIURL = defaultURL })
		latestReleaseAPIURL = srv.URL

		_, err := latestReleaseFromAPI(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
	})

	t.Run("malformed JSON → error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{not json`))
		}))
		defer srv.Close()
		t.Cleanup(func() { latestReleaseAPIURL = defaultURL })
		latestReleaseAPIURL = srv.URL

		_, err := latestReleaseFromAPI(context.Background())
		assert.Error(t, err)
	})
}

func TestLatestReleaseFromRedirect(t *testing.T) {
	const defaultURL = "https://github.com/JetBrains/teamcity-cli/releases/latest"

	t.Run("302 → version from Location", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodHead, r.Method)
			w.Header().Set("Location", "https://github.com/JetBrains/teamcity-cli/releases/tag/v1.5.0")
			w.WriteHeader(http.StatusFound)
		}))
		defer srv.Close()
		t.Cleanup(func() { latestReleaseRedirectURL = defaultURL })
		latestReleaseRedirectURL = srv.URL

		got, err := latestReleaseFromRedirect(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "1.5.0", got.Version)
		assert.Equal(t, "https://github.com/JetBrains/teamcity-cli/releases/tag/v1.5.0", got.URL)
	})

	t.Run("301 also accepted", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", "https://example.com/releases/tag/v2.0.0")
			w.WriteHeader(http.StatusMovedPermanently)
		}))
		defer srv.Close()
		t.Cleanup(func() { latestReleaseRedirectURL = defaultURL })
		latestReleaseRedirectURL = srv.URL

		got, err := latestReleaseFromRedirect(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", got.Version)
	})

	t.Run("200 (no redirect) → error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		t.Cleanup(func() { latestReleaseRedirectURL = defaultURL })
		latestReleaseRedirectURL = srv.URL

		_, err := latestReleaseFromRedirect(context.Background())
		assert.Error(t, err)
	})

	t.Run("missing Location header → error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusFound)
		}))
		defer srv.Close()
		t.Cleanup(func() { latestReleaseRedirectURL = defaultURL })
		latestReleaseRedirectURL = srv.URL

		_, err := latestReleaseFromRedirect(context.Background())
		assert.Error(t, err)
	})
}

func TestLatestRelease_FallbackOnAPIFailure(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden) // simulate rate limit
	}))
	defer apiSrv.Close()
	redirectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://github.com/x/y/releases/tag/v9.9.9")
		w.WriteHeader(http.StatusFound)
	}))
	defer redirectSrv.Close()

	origAPI, origRedirect := latestReleaseAPIURL, latestReleaseRedirectURL
	t.Cleanup(func() { latestReleaseAPIURL, latestReleaseRedirectURL = origAPI, origRedirect })
	latestReleaseAPIURL = apiSrv.URL
	latestReleaseRedirectURL = redirectSrv.URL

	got, err := LatestRelease(context.Background())
	require.NoError(t, err, "fallback should rescue from API failure")
	assert.Equal(t, "9.9.9", got.Version)
}

func TestLatestRelease_BothPathsFail(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer apiSrv.Close()
	redirectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer redirectSrv.Close()

	origAPI, origRedirect := latestReleaseAPIURL, latestReleaseRedirectURL
	t.Cleanup(func() { latestReleaseAPIURL, latestReleaseRedirectURL = origAPI, origRedirect })
	latestReleaseAPIURL = apiSrv.URL
	latestReleaseRedirectURL = redirectSrv.URL

	_, err := LatestRelease(context.Background())
	require.Error(t, err)
	// Error must mention both failures, not just one.
	assert.Contains(t, err.Error(), "fallback failed")
}
