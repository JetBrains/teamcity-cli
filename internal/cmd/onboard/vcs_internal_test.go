package onboard

import (
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// onboardedProjects is the onboarded project and its ancestors, as runOnboard passes them.
var onboardedProjects = []string{"Instance_App", "Instance", rootProjectID}

// vcsRootWithURL is a root defined in Instance_App unless inProject overrides it.
func vcsRootWithURL(id, url string, inProject ...string) api.VcsRoot {
	project := "Instance_App"
	if len(inProject) > 0 {
		project = inProject[0]
	}
	return api.VcsRoot{ID: id, Project: &api.Project{ID: project}, Properties: &api.PropertyList{Property: []api.Property{
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
		{"root without properties", remote, []api.VcsRoot{{ID: "r1", Project: &api.Project{ID: "Instance_App"}}}, false},
		{"no roots", remote, nil, false},
		{"root in instance project is usable", remote, []api.VcsRoot{vcsRootWithURL("r1", remote, "Instance")}, true},
		{"root in server root is usable", remote, []api.VcsRoot{vcsRootWithURL("r1", remote, rootProjectID)}, true},
		{"root in unrelated project is ignored", remote, []api.VcsRoot{vcsRootWithURL("r1", remote, "Instance_OtherTeam")}, false},
		{"root in a subproject is ignored", remote, []api.VcsRoot{vcsRootWithURL("r1", remote, "Instance_App_Nested")}, false},
		{"root without project is ignored", remote, []api.VcsRoot{{ID: "r1", Properties: vcsRootWithURL("r1", remote).Properties}}, false},
		{"unrelated match does not hide a usable one", remote, []api.VcsRoot{
			vcsRootWithURL("r1", remote, "Instance_OtherTeam"),
			vcsRootWithURL("r2", remote, "Instance"),
		}, true},
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

			got, err := isRepositoryAuthorized(client, repository{remoteURL: tc.remote}, onboardedProjects...)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsRepositoryAuthorizedNoRemote(t *testing.T) {
	client := newTestClient(t, nil) // any request fails the test

	got, err := isRepositoryAuthorized(client, repository{}, onboardedProjects...)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestIsRepositoryAuthorizedAPIError(t *testing.T) {
	client := newTestClient(t, map[string]http.HandlerFunc{
		"GET /app/rest/vcs-roots": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "forbidden", http.StatusForbidden)
		},
	})

	_, err := isRepositoryAuthorized(client, repository{remoteURL: "https://github.com/acme/app.git"}, onboardedProjects...)
	assert.ErrorContains(t, err, "list VCS roots")
}
