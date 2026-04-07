package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetVcsRoots(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/vcs-roots")
		assert.Contains(t, r.URL.RawQuery, "affectedProject")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(VcsRootList{
			Count: 1,
			VcsRoot: []VcsRoot{
				{ID: "vcs1", Name: "My Repo", VcsName: "jetbrains.git"},
			},
		})
	})

	result, err := client.GetVcsRoots(VcsRootsOptions{Project: "MyProject"})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Count)
	assert.Equal(t, "vcs1", result.VcsRoot[0].ID)
	assert.Equal(t, "jetbrains.git", result.VcsRoot[0].VcsName)
}

func TestGetVcsRoot(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/vcs-roots/id:vcs1")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(VcsRoot{
			ID:      "vcs1",
			Name:    "My Repo",
			VcsName: "jetbrains.git",
			Project: &Project{ID: "P1"},
			Properties: &PropertyList{
				Property: []Property{
					{Name: "url", Value: "https://github.com/org/repo"},
					{Name: "branch", Value: "refs/heads/main"},
					{Name: "secure:password"},
				},
			},
		})
	})

	root, err := client.GetVcsRoot("vcs1")
	require.NoError(t, err)
	assert.Equal(t, "My Repo", root.Name)
	assert.Equal(t, "P1", root.Project.ID)
	assert.Len(t, root.Properties.Property, 3)
}

func TestDeleteVcsRoot(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/vcs-roots/id:vcs1")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteVcsRoot("vcs1")
	require.NoError(t, err)
}
