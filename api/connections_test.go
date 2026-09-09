package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProjectConnections(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		if !strings.HasSuffix(r.URL.Path, "/projectFeatures") {
			_ = json.NewEncoder(w).Encode(Project{ID: "MyProject"})
			return
		}
		assert.Contains(t, r.URL.RawQuery, "locator=type:OAuthProvider")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ProjectFeatureList{
			ProjectFeature: []ProjectFeature{
				{ID: "PROJECT_EXT_1", Type: "OAuthProvider", Properties: &PropertyList{
					Property: []Property{{Name: "providerType", Value: "GitHub"}},
				}},
			},
		})
	})

	got, err := client.GetProjectConnections("MyProject")
	require.NoError(t, err)
	assert.Len(t, got.ProjectFeature, 1)
	assert.Equal(t, "PROJECT_EXT_1", got.ProjectFeature[0].ID)
}

func TestCreateProjectFeature(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/projects/id:MyProject/projectFeatures"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var sent ProjectFeature
		require.NoError(t, json.Unmarshal(body, &sent))
		assert.Equal(t, "OAuthProvider", sent.Type)
		require.NotNil(t, sent.Properties)
		// Ensure required props round-trip in the body.
		var seenType, seenClientID, seenSecret bool
		for _, p := range sent.Properties.Property {
			switch p.Name {
			case "providerType":
				seenType = p.Value == "GitHub"
			case "clientId":
				seenClientID = p.Value == "abc"
			case "secure:clientSecret":
				seenSecret = p.Value == "xyz"
			}
		}
		assert.True(t, seenType, "providerType should be sent")
		assert.True(t, seenClientID, "clientId should be sent")
		assert.True(t, seenSecret, "secure:clientSecret should be sent")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ProjectFeature{
			ID:   "PROJECT_EXT_42",
			Type: "OAuthProvider",
			Properties: &PropertyList{
				Property: []Property{
					{Name: "providerType", Value: "GitHub"},
					{Name: "clientId", Value: "abc"},
					{Name: "secure:clientSecret"}, // server strips the value on read-back
				},
			},
		})
	})

	feat := ProjectFeature{
		Type: "OAuthProvider",
		Properties: &PropertyList{
			Property: []Property{
				{Name: "providerType", Value: "GitHub"},
				{Name: "clientId", Value: "abc"},
				{Name: "secure:clientSecret", Value: "xyz"},
			},
		},
	}
	created, err := client.CreateProjectFeature("MyProject", feat)
	require.NoError(t, err)
	assert.Equal(t, "PROJECT_EXT_42", created.ID)
	assert.Equal(t, "OAuthProvider", created.Type)
}

func TestDeleteProjectFeature(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/projects/id:MyProject/projectFeatures/id:PROJECT_EXT_42"))
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteProjectFeature("MyProject", "PROJECT_EXT_42")
	require.NoError(t, err)
}

func TestDeleteProjectFeatureNotFound(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"message":"Nothing is found"}]}`))
	})

	err := client.DeleteProjectFeature("MyProject", "PROJECT_EXT_99")
	require.Error(t, err)
}

func TestGetProjectConnectionsInherited(t *testing.T) {
	t.Parallel()
	requests := []string{}
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/app/rest/projects/id:Child":
			_ = json.NewEncoder(w).Encode(Project{ID: "Child", ParentProjectID: "Parent"})
		case "/app/rest/projects/id:Parent":
			_ = json.NewEncoder(w).Encode(Project{ID: "Parent", ParentProjectID: "_Root"})
		default:
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/app/rest/projects/id:"), "/projectFeatures")
			_ = json.NewEncoder(w).Encode(ProjectFeatureList{ProjectFeature: []ProjectFeature{{ID: id, Type: "OAuthProvider"}}})
		}
	})
	got, err := client.GetProjectConnections("Child")
	require.NoError(t, err)
	require.Len(t, got.ProjectFeature, 3)
	assert.Equal(t, 3, got.Count)
	assert.Equal(t, "Child", got.ProjectFeature[0].ID)
	assert.Equal(t, "Parent", got.ProjectFeature[1].ID)
	assert.Equal(t, "_Root", got.ProjectFeature[2].ID)
	assert.Len(t, requests, 5)
}

func TestGetProjectConnectionsHierarchyError(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/projectFeatures") {
					_ = json.NewEncoder(w).Encode(ProjectFeatureList{ProjectFeature: []ProjectFeature{}})
					return
				}
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(Project{ID: "Child", ParentProjectID: "Child"})
			})
			got, err := client.GetProjectConnections("Child")
			require.Error(t, err)
			assert.Nil(t, got)
			if status == http.StatusOK {
				assert.Contains(t, err.Error(), "cycle")
			} else {
				assert.Contains(t, err.Error(), "failed to resolve parent")
			}
		})
	}
}
