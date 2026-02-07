package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetVcsRoots(T *testing.T) {
	T.Parallel()

	T.Run("success", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.True(t, strings.HasPrefix(r.URL.Path, "/app/rest/vcs-roots"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VcsRootList{
				Count: 2,
				VcsRoots: []VcsRoot{
					{ID: "Project_GitRepo", Name: "Git Repo", VcsName: "jetbrains.git"},
					{ID: "Project_P4Depot", Name: "Perforce Depot", VcsName: "perforce"},
				},
			})
		})

		roots, err := client.GetVcsRoots(VcsRootOptions{Project: "TestProject"})
		require.NoError(t, err)
		assert.Equal(t, 2, roots.Count)
		assert.Equal(t, "jetbrains.git", roots.VcsRoots[0].VcsName)
		assert.Equal(t, "perforce", roots.VcsRoots[1].VcsName)
	})

	T.Run("empty", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VcsRootList{Count: 0, VcsRoots: nil})
		})

		roots, err := client.GetVcsRoots(VcsRootOptions{})
		require.NoError(t, err)
		assert.Equal(t, 0, roots.Count)
	})
}

func TestGetVcsRoot(T *testing.T) {
	T.Parallel()

	T.Run("git root", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VcsRoot{
				ID:      "Project_GitRepo",
				Name:    "Git Repository",
				VcsName: "jetbrains.git",
				Properties: PropertyList{
					Property: []Property{
						{Name: "url", Value: "https://github.com/example/repo.git"},
						{Name: "branch", Value: "refs/heads/main"},
					},
				},
			})
		})

		root, err := client.GetVcsRoot("Project_GitRepo")
		require.NoError(t, err)
		assert.Equal(t, "jetbrains.git", root.VcsName)
		assert.Len(t, root.Properties.Property, 2)
	})

	T.Run("perforce root", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VcsRoot{
				ID:      "Project_P4Depot",
				Name:    "Perforce Stream",
				VcsName: "perforce",
				Properties: PropertyList{
					Property: []Property{
						{Name: "port", Value: "ssl:perforce.example.com:1666"},
						{Name: "user", Value: "buildbot"},
						{Name: "stream", Value: "//depot/main"},
					},
				},
			})
		})

		root, err := client.GetVcsRoot("Project_P4Depot")
		require.NoError(t, err)
		assert.Equal(t, "perforce", root.VcsName)
		assert.Len(t, root.Properties.Property, 3)
	})

	T.Run("not found", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors":[{"message":"VCS root not found"}]}`))
		})

		_, err := client.GetVcsRoot("NonExistent")
		assert.Error(t, err)
	})
}

func TestCreateVcsRoot(T *testing.T) {
	T.Parallel()

	T.Run("create git root", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/app/rest/vcs-roots", r.URL.Path)

			body, _ := io.ReadAll(r.Body)
			var payload createVcsRootPayload
			require.NoError(t, json.Unmarshal(body, &payload))

			assert.Equal(t, "jetbrains.git", payload.VcsName)
			assert.Equal(t, "My Git Repo", payload.Name)
			assert.NotNil(t, payload.Project)
			assert.Equal(t, "TestProject", payload.Project.ID)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(VcsRoot{
				ID:      "TestProject_MyGitRepo",
				Name:    "My Git Repo",
				VcsName: "jetbrains.git",
			})
		}))
		t.Cleanup(server.Close)
		client := NewClient(server.URL, "test-token")

		root, err := client.CreateVcsRoot(CreateVcsRootRequest{
			Name:      "My Git Repo",
			VcsName:   "jetbrains.git",
			ProjectID: "TestProject",
			Properties: NewGitVcsRootProperties(
				"https://github.com/example/repo.git",
				"refs/heads/main",
			),
		})
		require.NoError(t, err)
		assert.Equal(t, "TestProject_MyGitRepo", root.ID)
	})

	T.Run("create perforce root", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)

			body, _ := io.ReadAll(r.Body)
			var payload createVcsRootPayload
			require.NoError(t, json.Unmarshal(body, &payload))

			assert.Equal(t, "perforce", payload.VcsName)
			assert.Equal(t, "P4 Stream", payload.Name)

			// Verify Perforce-specific properties
			props := make(map[string]string)
			for _, p := range payload.Properties.Property {
				props[p.Name] = p.Value
			}
			assert.Equal(t, "ssl:p4.example.com:1666", props["port"])
			assert.Equal(t, "buildbot", props["user"])
			assert.Equal(t, "//depot/main", props["stream"])

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VcsRoot{
				ID:      "TestProject_P4Stream",
				Name:    "P4 Stream",
				VcsName: "perforce",
			})
		}))
		t.Cleanup(server.Close)
		client := NewClient(server.URL, "test-token")

		root, err := client.CreateVcsRoot(CreateVcsRootRequest{
			Name:      "P4 Stream",
			VcsName:   "perforce",
			ProjectID: "TestProject",
			Properties: NewPerforceVcsRootProperties(
				"ssl:p4.example.com:1666",
				"buildbot",
				"secret123",
				"//depot/main",
			),
		})
		require.NoError(t, err)
		assert.Equal(t, "perforce", root.VcsName)
	})
}

func TestDeleteVcsRoot(T *testing.T) {
	T.Parallel()

	T.Run("success", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "DELETE", r.Method)
			assert.True(t, strings.Contains(r.URL.Path, "/vcs-roots/id:"))
			w.WriteHeader(http.StatusNoContent)
		})

		err := client.DeleteVcsRoot("TestProject_GitRepo")
		require.NoError(t, err)
	})

	T.Run("not found", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors":[{"message":"VCS root not found"}]}`))
		})

		err := client.DeleteVcsRoot("NonExistent")
		assert.Error(t, err)
	})
}

func TestVcsRootExists(T *testing.T) {
	T.Parallel()

	T.Run("exists", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VcsRoot{ID: "TestVcsRoot", Name: "Test"})
		})

		assert.True(t, client.VcsRootExists("TestVcsRoot"))
	})

	T.Run("not exists", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		assert.False(t, client.VcsRootExists("NonExistent"))
	})
}

func TestAttachVcsRoot(T *testing.T) {
	T.Parallel()

	T.Run("success", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.True(t, strings.HasSuffix(r.URL.Path, "/vcs-root-entries"))

			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), "TestProject_P4Root")

			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)
		client := NewClient(server.URL, "test-token")

		err := client.AttachVcsRoot("Sandbox_Build", "TestProject_P4Root")
		require.NoError(t, err)
	})
}

func TestNewPerforceVcsRootProperties(T *testing.T) {
	T.Parallel()

	T.Run("with stream", func(t *testing.T) {
		t.Parallel()

		props := NewPerforceVcsRootProperties("p4.example.com:1666", "builder", "pass", "//depot/main")

		names := make(map[string]string)
		for _, p := range props.Property {
			names[p.Name] = p.Value
		}
		assert.Equal(t, "p4.example.com:1666", names["port"])
		assert.Equal(t, "builder", names["user"])
		assert.Equal(t, "pass", names["secure:passwd"])
		assert.Equal(t, "//depot/main", names["stream"])
		assert.Equal(t, "false", names["use-client"])
	})

	T.Run("without password", func(t *testing.T) {
		t.Parallel()

		props := NewPerforceVcsRootProperties("p4:1666", "user", "", "//depot/dev")

		names := make(map[string]string)
		for _, p := range props.Property {
			names[p.Name] = p.Value
		}
		assert.Equal(t, "p4:1666", names["port"])
		_, hasPassword := names["secure:passwd"]
		assert.False(t, hasPassword)
	})
}

func TestNewGitVcsRootProperties(T *testing.T) {
	T.Parallel()

	T.Run("with branch", func(t *testing.T) {
		t.Parallel()

		props := NewGitVcsRootProperties("https://github.com/example/repo.git", "refs/heads/main")

		names := make(map[string]string)
		for _, p := range props.Property {
			names[p.Name] = p.Value
		}
		assert.Equal(t, "https://github.com/example/repo.git", names["url"])
		assert.Equal(t, "refs/heads/main", names["branch"])
	})

	T.Run("without branch", func(t *testing.T) {
		t.Parallel()

		props := NewGitVcsRootProperties("https://github.com/example/repo.git", "")

		assert.Len(t, props.Property, 1)
		assert.Equal(t, "url", props.Property[0].Name)
	})
}
