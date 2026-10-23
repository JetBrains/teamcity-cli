package onboard

import (
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testConnectionPath = "POST /app/pipeline/repository/testConnection"
	vcsRootsPath       = "POST /app/rest/vcs-roots"
)

// connectionRoutes serves an OAuth connection list for the Instance project (and an empty _Root above it).
func connectionRoutes(features ...api.ProjectFeature) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /app/rest/projects/id:Instance/projectFeatures": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, api.ProjectFeatureList{Count: len(features), ProjectFeature: features})
		},
		"GET /app/rest/projects/id:Instance": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, api.Project{ID: "Instance", ParentProjectID: "_Root"})
		},
		"GET /app/rest/projects/id:_Root/projectFeatures": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, api.ProjectFeatureList{})
		},
	}
}

// oauthConnection is a connection feature; extra properties (e.g. a self-hosted server URL) are appended.
func oauthConnection(id, providerType string, extra ...api.Property) api.ProjectFeature {
	props := append([]api.Property{
		{Name: "displayName", Value: id},
		{Name: "providerType", Value: providerType},
	}, extra...)
	return api.ProjectFeature{ID: id, Type: "OAuthProvider", Properties: &api.PropertyList{Property: props}}
}

func TestHTTPSURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"git@github.com:acme/app.git", "https://github.com/acme/app.git"},
		{"ssh://git@github.com/acme/app.git", "https://github.com/acme/app.git"},
		{"https://github.com/acme/app", "https://github.com/acme/app.git"},
		{"https://github.com/acme/app.git", "https://github.com/acme/app.git"},
		{"not-a-url", "not-a-url"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, httpsURL(tc.in))
		})
	}
}

func TestDefaultBranch(t *testing.T) {
	t.Run("remote HEAD", func(t *testing.T) {
		remote := initRepoWithCommit(t, "trunk")
		assert.Equal(t, "refs/heads/trunk", defaultBranch(t.Context(), remote))
	})

	t.Run("unreachable remote falls back to main", func(t *testing.T) {
		assert.Equal(t, "refs/heads/main", defaultBranch(t.Context(), filepath.Join(t.TempDir(), "missing")))
	})
}

func TestCreateVcsRoot(t *testing.T) {
	var got api.VcsRoot
	client := newTestClient(t, map[string]http.HandlerFunc{
		vcsRootsPath: func(w http.ResponseWriter, r *http.Request) {
			decodeJSON(t, r, &got)
			writeJSON(w, api.VcsRoot{ID: "Instance_App_Root", Name: got.Name})
		},
	})
	f, out := testFactory()

	err := createVcsRoot(f, client, "Instance_App", "https://github.com/acme/app.git", "refs/heads/trunk", "PROJECT_EXT_2")
	require.NoError(t, err)

	assert.Equal(t, "acme/app", got.Name)
	assert.Equal(t, "jetbrains.git", got.VcsName)
	require.NotNil(t, got.Project)
	assert.Equal(t, "Instance_App", got.Project.ID)
	assert.Equal(t, "PROJECT_EXT_2", got.ConnectionID)
	require.NotNil(t, got.Properties)
	assert.Equal(t, "https://github.com/acme/app.git", property(got.Properties.Property, "url"))
	assert.Equal(t, "refs/heads/trunk", property(got.Properties.Property, "branch"))
	assert.Contains(t, out.String(), `Created VCS root "acme/app" (Instance_App_Root)`)
}

func TestEnsureConnectionAccess(t *testing.T) {
	conn := connection{id: "PROJECT_EXT_2", providerType: "GitHub", projectID: "Instance"}
	const repoURL = "https://github.com/acme/app.git"

	t.Run("access OK", func(t *testing.T) {
		client := newTestClient(t, map[string]http.HandlerFunc{
			testConnectionPath: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Instance", r.URL.Query().Get("parentProjectExtId"))
				var req api.TestConnectionRequest
				decodeJSON(t, r, &req)
				assert.Equal(t, repoURL, req.URL)
				assert.Equal(t, "PROJECT_EXT_2", req.ConnectionID)
				writeJSON(w, api.TestConnectionResult{Status: "OK"})
			},
		})
		f, _ := testFactory()

		ok, err := ensureConnectionAccess(f, client, repoURL, conn)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("non-authorization failure is returned without opening the browser", func(t *testing.T) {
		client := newTestClient(t, map[string]http.HandlerFunc{
			testConnectionPath: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, api.TestConnectionResult{Status: "ERROR", Errors: []api.TestConnectionError{{Message: "Repository not found"}}})
			},
		})
		f, out := testFactory()

		ok, err := ensureConnectionAccess(f, client, repoURL, conn)
		assert.False(t, ok)
		assert.ErrorContains(t, err, "Repository not found")
		assert.NotContains(t, out.String(), "/oauth/github/")
	})

	t.Run("older server skips the check", func(t *testing.T) {
		client := newTestClient(t, map[string]http.HandlerFunc{
			"GET /app/rest/server": func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, api.Server{VersionMajor: 2024, VersionMinor: 3})
			},
		})
		f, _ := testFactory()

		ok, err := ensureConnectionAccess(f, client, repoURL, conn)
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestAuthorizeRepository(t *testing.T) {
	t.Run("no remote does nothing", func(t *testing.T) {
		client := newTestClient(t, nil) // any request fails the test
		f, _ := testFactory()

		ok, err := authorizeRepository(f, client, repository{source: sourcePath, dir: t.TempDir()}, "Instance", "Instance_App")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("demo creates anonymous root in repo project without a connection lookup", func(t *testing.T) {
		remote := initRepoWithCommit(t, "main")
		var got api.VcsRoot
		client := newTestClient(t, map[string]http.HandlerFunc{
			vcsRootsPath: func(w http.ResponseWriter, r *http.Request) {
				decodeJSON(t, r, &got)
				writeJSON(w, api.VcsRoot{ID: "Instance_Demo_Root", Name: got.Name})
			},
		})
		f, _ := testFactory()

		ok, err := authorizeRepository(f, client, repository{source: sourceDemo, remoteURL: remote}, "Instance", "Instance_Demo")
		require.NoError(t, err)
		assert.True(t, ok)
		require.NotNil(t, got.Project)
		assert.Equal(t, "Instance_Demo", got.Project.ID)
		assert.Empty(t, got.ConnectionID)
		assert.Equal(t, "ANONYMOUS", property(got.Properties.Property, "authMethod"))
		assert.Equal(t, remote, property(got.Properties.Property, "url"))
		assert.Equal(t, "refs/heads/main", property(got.Properties.Property, "branch"))
	})

	t.Run("no VCS connection fails before any prompt", func(t *testing.T) {
		client := newTestClient(t, connectionRoutes(oauthConnection("PROJECT_EXT_9", "Docker")))
		f, _ := testFactory()

		ok, err := authorizeRepository(f, client, repository{source: sourcePath, remoteURL: "git@github.com:acme/app.git"}, "Instance", "Instance_App")
		assert.False(t, ok)
		ve, isValidation := errors.AsType[*api.ValidationError](err)
		require.True(t, isValidation, "want *api.ValidationError, got %T: %v", err, err)
		assert.Contains(t, ve.Error(), "no VCS connection")
	})
}
