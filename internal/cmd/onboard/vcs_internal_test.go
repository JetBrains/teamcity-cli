package onboard

import (
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func vcsRootWithURL(id, url string) api.VcsRoot {
	return api.VcsRoot{ID: id, Properties: &api.PropertyList{Property: []api.Property{
		{Name: "branch", Value: "refs/heads/main"},
		{Name: "url", Value: url},
	}}}
}

func TestIsRepositoryAuthorized(t *testing.T) {
	const remote = "https://github.com/acme/app.git"
	tests := []struct {
		name   string
		remote string
		roots  []api.VcsRoot
		want   bool
	}{
		{"same URL", remote, []api.VcsRoot{vcsRootWithURL("r1", remote)}, true},
		{"ssh remote vs https root", "git@github.com:acme/app.git", []api.VcsRoot{vcsRootWithURL("r1", "https://github.com/acme/app")}, true},
		{"fork is not a match", remote, []api.VcsRoot{vcsRootWithURL("r1", "https://github.com/someone/acme/app.git")}, false},
		{"lookalike is not a match", remote, []api.VcsRoot{vcsRootWithURL("r1", "https://github.com/acme/app-legacy.git")}, false},
		{"other host", remote, []api.VcsRoot{vcsRootWithURL("r1", "https://gitlab.com/acme/app.git")}, false},
		{"root without properties", remote, []api.VcsRoot{{ID: "r1"}}, false},
		{"no roots", remote, nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, map[string]http.HandlerFunc{
				"GET /app/rest/vcs-roots": func(w http.ResponseWriter, r *http.Request) {
					assert.Contains(t, r.URL.Query().Get("locator"), "property:(name:url,value:acme/app,matchType:contains)")
					assert.Contains(t, r.URL.Query().Get("fields"), "properties(property(name,value))")
					writeJSON(w, api.VcsRootList{Count: len(tc.roots), VcsRoot: tc.roots})
				},
			})

			got, err := isRepositoryAuthorized(client, repository{remoteURL: tc.remote})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsRepositoryAuthorizedNoRemote(t *testing.T) {
	client := newTestClient(t, nil) // any request fails the test

	got, err := isRepositoryAuthorized(client, repository{})
	require.NoError(t, err)
	assert.False(t, got)
}

func TestIsRepositoryAuthorizedAPIError(t *testing.T) {
	client := newTestClient(t, map[string]http.HandlerFunc{
		"GET /app/rest/vcs-roots": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "forbidden", http.StatusForbidden)
		},
	})

	_, err := isRepositoryAuthorized(client, repository{remoteURL: "https://github.com/acme/app.git"})
	assert.ErrorContains(t, err, "list VCS roots")
}
