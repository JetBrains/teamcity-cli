package onboard

import (
	"errors"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindInstanceProject(t *testing.T) {
	t.Run("direct child of root", func(t *testing.T) {
		client := newTestClient(t, map[string]http.HandlerFunc{
			"GET /app/rest/projects": func(w http.ResponseWriter, r *http.Request) {
				locator := r.URL.Query().Get("locator")
				assert.Contains(t, locator, "parentProject:_Root")
				assert.Contains(t, locator, "archived:false")
				writeJSON(w, api.ProjectList{Count: 1, Projects: []api.Project{{ID: "Instance"}}})
			},
		})

		got, err := findInstanceProject(client)
		require.NoError(t, err)
		assert.Equal(t, "Instance", got)
	})

	t.Run("no children", func(t *testing.T) {
		client := newTestClient(t, map[string]http.HandlerFunc{
			"GET /app/rest/projects": func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, api.ProjectList{})
			},
		})

		_, err := findInstanceProject(client)
		_, ok := errors.AsType[*api.ValidationError](err)
		assert.True(t, ok, "want *api.ValidationError, got %T: %v", err, err)
	})
}

func TestEnsureRepoProject(t *testing.T) {
	repo := repository{remoteURL: "git@github.com:acme/app.git"}

	t.Run("reuses existing project", func(t *testing.T) {
		client := newTestClient(t, map[string]http.HandlerFunc{
			"GET /app/rest/projects": func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Query().Get("locator"), "parentProject:Instance")
				writeJSON(w, api.ProjectList{Count: 2, Projects: []api.Project{
					{ID: "Instance_Other", Name: "other"},
					{ID: "Instance_App", Name: "app"},
				}})
			},
		})
		f, _ := testFactory()

		got, err := ensureRepoProject(f, client, "Instance", repo)
		require.NoError(t, err)
		assert.Equal(t, "Instance_App", got)
	})

	t.Run("creates missing project under instance", func(t *testing.T) {
		client := newTestClient(t, map[string]http.HandlerFunc{
			"GET /app/rest/projects": func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, api.ProjectList{Count: 1, Projects: []api.Project{{ID: "Instance_Other", Name: "other"}}})
			},
			"POST /app/rest/projects": func(w http.ResponseWriter, r *http.Request) {
				var req api.CreateProjectRequest
				decodeJSON(t, r, &req)
				assert.Equal(t, "app", req.Name)
				require.NotNil(t, req.ParentProject)
				assert.Equal(t, "Instance", req.ParentProject.ID)
				assert.Empty(t, req.ID, "server generates the ID")
				writeJSON(w, api.Project{ID: "Instance_App", Name: req.Name})
			},
		})
		f, out := testFactory()

		got, err := ensureRepoProject(f, client, "Instance", repo)
		require.NoError(t, err)
		assert.Equal(t, "Instance_App", got)
		assert.Contains(t, out.String(), `Created project "app"`)
	})

	t.Run("create failure", func(t *testing.T) {
		client := newTestClient(t, map[string]http.HandlerFunc{
			"GET /app/rest/projects": func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, api.ProjectList{})
			},
			"POST /app/rest/projects": func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			},
		})
		f, _ := testFactory()

		_, err := ensureRepoProject(f, client, "Instance", repo)
		assert.ErrorContains(t, err, `failed to create project "app"`)
	})
}

func TestRepoName(t *testing.T) {
	tests := []struct {
		name string
		repo repository
		want string
	}{
		{"ssh remote", repository{remoteURL: "git@github.com:acme/app.git"}, "app"},
		{"https remote", repository{remoteURL: "https://github.com/spring-projects/spring-petclinic"}, "spring-petclinic"},
		{"nested group", repository{remoteURL: "https://gitlab.com/acme/platform/api.git"}, "api"},
		{"local path remote", repository{remoteURL: "/srv/git/tool.git"}, "tool"},
		{"no remote uses dir", repository{dir: "/home/me/projects/scratch"}, "scratch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, repoName(tc.repo))
		})
	}
}
