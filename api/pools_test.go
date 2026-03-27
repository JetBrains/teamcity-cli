package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAgentPools(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/agentPools")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PoolList{
			Count: 2,
			Pools: []Pool{{ID: 0, Name: "Default"}, {ID: 1, Name: "Linux"}},
		})
	})

	result, err := client.GetAgentPools(nil)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Count)
}

func TestGetAgentPool(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/agentPools/id:1")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Pool{ID: 1, Name: "Linux", MaxAgents: 10})
	})

	pool, err := client.GetAgentPool(1)
	require.NoError(t, err)
	assert.Equal(t, "Linux", pool.Name)
	assert.Equal(t, 10, pool.MaxAgents)
}

func TestAddProjectToPool(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/agentPools/id:1/projects")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.AddProjectToPool(1, "MyProject")
	require.NoError(t, err)
}

func TestRemoveProjectFromPool(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/agentPools/id:1/projects/id:MyProject")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.RemoveProjectFromPool(1, "MyProject")
	require.NoError(t, err)
}

func TestSetAgentPool(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/agents/id:5/pool")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.SetAgentPool(5, 1)
	require.NoError(t, err)
}
