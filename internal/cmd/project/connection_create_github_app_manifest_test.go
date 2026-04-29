package project

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildManifest(t *testing.T) {
	t.Parallel()

	body, err := buildManifest("TC Sandbox@tc.example.com", "https://tc.example.com", "http://localhost:53219/cb", "Sandbox")
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))

	assert.Equal(t, "TC Sandbox@tc.example.com", m["name"])
	assert.Equal(t, "https://tc.example.com/admin/editProject.html?projectId=Sandbox&tab=oauthConnections", m["url"])
	assert.Equal(t, "http://localhost:53219/cb", m["redirect_url"])
	assert.Equal(t, []any{"https://tc.example.com/oauth/githubapp/accessToken.html"}, m["callback_urls"])
	assert.Equal(t, false, m["public"])

	perms, _ := m["default_permissions"].(map[string]any)
	assert.Equal(t, "write", perms["contents"])
	assert.Equal(t, "read", perms["metadata"])
	assert.Equal(t, "write", perms["pull_requests"])
	assert.Equal(t, "write", perms["statuses"])
	assert.Equal(t, "write", perms["checks"])
}

func TestBuildManifestStripsTrailingSlashAndEscapesProject(t *testing.T) {
	t.Parallel()

	body, err := buildManifest("Backend", "https://tc.example.com/", "http://localhost:5/cb", "Project With Spaces")
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	assert.Equal(t, "https://tc.example.com/admin/editProject.html?projectId=Project+With+Spaces&tab=oauthConnections", m["url"])
	assert.Equal(t, []any{"https://tc.example.com/oauth/githubapp/accessToken.html"}, m["callback_urls"])
}

func TestManifestSubmitURL(t *testing.T) {
	prev := githubURLHost
	t.Cleanup(func() { githubURLHost = prev })
	githubURLHost = "https://example.invalid"

	assert.Equal(t, "https://example.invalid/settings/apps/new", manifestSubmitURL(""))
	assert.Equal(t, "https://example.invalid/organizations/myorg/settings/apps/new", manifestSubmitURL("myorg"))
}

func TestBuildStartHandler(t *testing.T) {
	prev := githubURLHost
	t.Cleanup(func() { githubURLHost = prev })
	githubURLHost = "https://example.invalid"

	manifest := []byte(`{"name":"Test"}`)
	h := buildStartHandler("myorg", manifest, "expected-state")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/start", nil)
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `action="https://example.invalid/organizations/myorg/settings/apps/new"`)
	assert.Contains(t, body, `name="manifest"`)
	assert.Contains(t, body, `name="state"`)
	assert.Contains(t, body, "expected-state")
	assert.Contains(t, body, "document.forms[0].submit()")
}

func TestExchangeManifestCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/app-manifests/abc123/conversions", r.URL.Path)
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		_ = json.NewEncoder(w).Encode(manifestCreds{
			AppID:        12345,
			Slug:         "tc-test",
			ClientID:     "Iv1.testid",
			ClientSecret: "testsecret",
			PEM:          "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			HTMLURL:      "https://github.com/apps/tc-test",
		})
	}))
	t.Cleanup(server.Close)

	prev := githubAPIHost
	t.Cleanup(func() { githubAPIHost = prev })
	githubAPIHost = server.URL

	creds, err := exchangeManifestCode(t.Context(), "abc123")
	require.NoError(t, err)
	assert.EqualValues(t, 12345, creds.AppID)
	assert.Equal(t, "tc-test", creds.Slug)
	assert.Equal(t, "Iv1.testid", creds.ClientID)
	assert.Contains(t, creds.PEM, "BEGIN RSA PRIVATE KEY")
}

func TestExchangeManifestCodeMissingFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":12345,"slug":"x"}`))
	}))
	t.Cleanup(server.Close)

	prev := githubAPIHost
	t.Cleanup(func() { githubAPIHost = prev })
	githubAPIHost = server.URL

	_, err := exchangeManifestCode(t.Context(), "abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required fields")
}

func TestExchangeManifestCodeServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Code expired"}`))
	}))
	t.Cleanup(server.Close)

	prev := githubAPIHost
	t.Cleanup(func() { githubAPIHost = prev })
	githubAPIHost = server.URL

	_, err := exchangeManifestCode(t.Context(), "expired-code")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestDefaultGitHubAppName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		project, server, want string
	}{
		{"Sandbox", "https://cli.teamcity.com", "TC Sandbox@cli.teamcity.com"},
		{"Sandbox", "https://tc.example.com:8111", "TC Sandbox@tc.example.com:8111"},
		{"_Root", "https://cli.teamcity.com", "TC _Root@cli.teamcity.com"},
		{"VeryLongProjectName", "https://cli.teamcity.com", "TC VeryLongProjectName@cli.teamcit"}, // 34-char truncation
	}
	for _, tc := range tests {
		got := defaultGitHubAppName(tc.project, tc.server)
		assert.Equal(t, tc.want, got, "project=%q server=%q", tc.project, tc.server)
		assert.LessOrEqual(t, len(got), 34)
	}
}

func TestGitHubAppName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want string
	}{
		{"Backend", "Backend"},
		{"github-test", "TC github-test"},
		{"GitHub Sandbox", "TC GitHub Sandbox"},
		{"gist-something", "TC gist-something"},
		{"github/tiulpin", "TC github-tiulpin"},
		{"backend/staging", "backend-staging"},
		{"This-name-is-way-too-long-for-the-34-char-limit", "This-name-is-way-too-long-for-the-"},
		{"  spaced  ", "spaced"},
	}
	for _, tc := range tests {
		got := githubAppName(tc.in)
		assert.Equal(t, tc.want, got, "input=%q", tc.in)
		assert.LessOrEqual(t, len(got), 34, "input=%q produced %q (%d chars)", tc.in, got, len(got))
	}
}
