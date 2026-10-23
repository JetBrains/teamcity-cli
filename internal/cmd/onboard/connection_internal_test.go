package onboard

import (
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListConnections(t *testing.T) {
	t.Run("keeps VCS-capable providers only", func(t *testing.T) {
		client := newTestClient(t, connectionRoutes(
			oauthConnection("PROJECT_EXT_1", "GitHub"),
			oauthConnection("PROJECT_EXT_2", "Docker"),
			oauthConnection("PROJECT_EXT_3", "GHE", api.Property{Name: "gitHubUrl", Value: "https://GHE.acme.internal/"}),
		))

		got, err := listConnections(client, "Instance")
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, connection{id: "PROJECT_EXT_1", name: "PROJECT_EXT_1", providerType: "GitHub", projectID: "Instance"}, got[0])
		assert.Equal(t, "GHE", got[1].providerType)
		assert.Equal(t, []string{"ghe.acme.internal"}, got[1].serverHosts, "URL-valued properties feed self-hosted matching, lowercased")
	})

	t.Run("API error", func(t *testing.T) {
		client := newTestClient(t, map[string]http.HandlerFunc{
			"GET /app/rest/projects/id:Instance/projectFeatures": func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			},
		})

		_, err := listConnections(client, "Instance")
		assert.ErrorContains(t, err, "list connections in Instance")
	})
}

func TestMatchConnections(t *testing.T) {
	github := connection{id: "gh", providerType: "GitHub"}
	githubApp := connection{id: "gh-app", providerType: "GitHubApp"}
	gitlab := connection{id: "gl", providerType: "GitLabCom"}
	ghe := connection{id: "ghe", providerType: "GHE", serverHosts: []string{"ghe.acme.internal"}}
	selfGitlab := connection{id: "gl-self", providerType: "GitLabCEorEE", serverHosts: []string{"git.acme.internal"}}
	azure := connection{id: "az", providerType: "AzureDevOps"}
	all := []connection{github, githubApp, gitlab, ghe, selfGitlab, azure}

	ids := func(conns []connection) []string {
		out := []string{}
		for _, c := range conns {
			out = append(out, c.id)
		}
		return out
	}

	tests := []struct {
		remote string
		want   []string
	}{
		{"git@github.com:acme/app.git", []string{"gh", "gh-app"}},
		{"https://GitHub.com/acme/app", []string{"gh", "gh-app"}},
		{"https://gitlab.com/acme/app.git", []string{"gl"}},
		{"https://ghe.acme.internal/acme/app.git", []string{"ghe"}},
		{"git@git.acme.internal:acme/app.git", []string{"gl-self"}},
		{"git@ssh.dev.azure.com:v3/acme/proj/app", []string{"az"}},
		{"https://bitbucket.org/acme/app.git", []string{}},
		{"not-a-url", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.remote, func(t *testing.T) {
			assert.Equal(t, tc.want, ids(matchConnections(all, tc.remote)))
		})
	}
}

func TestChooseConnectionSingleMatchSkipsPrompt(t *testing.T) {
	f, _ := testFactory() // non-interactive: any prompt would fail
	conns := []connection{
		{id: "gh", providerType: "GitHub"},
		{id: "gl", providerType: "GitLabCom"},
	}

	got, err := chooseConnection(f, conns, "git@github.com:acme/app.git")
	require.NoError(t, err)
	assert.Equal(t, "gh", got.id)
}

func TestAuthorizeURL(t *testing.T) {
	tests := []struct {
		providerType string
		want         string
	}{
		{"GitHub", "https://tc.example/oauth/github/repositories.html?projectId=Instance+%2B&connectionId=PROJECT_EXT_1&updateToken=true&showMode=popup"},
		{"GitHubApp", "https://tc.example/oauth/githubapp/repositories.html?projectId=Instance+%2B&connectionId=PROJECT_EXT_1&updateToken=true&showMode=popup"},
		{"GitLabCEorEE", "https://tc.example/oauth/gitlab/repositories.html?projectId=Instance+%2B&connectionId=PROJECT_EXT_1&updateToken=true&showMode=popup"},
		{"Docker", ""},
	}
	for _, tc := range tests {
		t.Run(tc.providerType, func(t *testing.T) {
			conn := connection{id: "PROJECT_EXT_1", providerType: tc.providerType, projectID: "Instance +"}
			assert.Equal(t, tc.want, authorizeURL("https://tc.example", conn))
		})
	}
}
