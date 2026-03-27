package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBuildQueue(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildQueue")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildQueue{
			Count:  1,
			Builds: []QueuedBuild{{ID: 100, State: "queued"}},
		})
	})

	result, err := client.GetBuildQueue(QueueOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Count)
}

func TestGetBuildQueueWithFilter(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "buildType%3Abt1")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildQueue{Count: 0})
	})

	_, err := client.GetBuildQueue(QueueOptions{BuildTypeID: "bt1"})
	require.NoError(t, err)
}

func TestRemoveFromQueue(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildQueue/id:100")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.RemoveFromQueue("100")
	require.NoError(t, err)
}

func TestSetQueuedBuildPosition(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildQueue/order/100")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.SetQueuedBuildPosition("100", 5)
	require.NoError(t, err)
}

func TestMoveQueuedBuildToTop(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/buildQueue/order/100")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.MoveQueuedBuildToTop("100")
	require.NoError(t, err)
}

func TestGetQueuedBuildApprovalInfo(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/buildQueue/id:100/approval")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApprovalInfo{Status: "waitingForApproval", CanBeApprovedByCurrentUser: true})
	})

	info, err := client.GetQueuedBuildApprovalInfo("100")
	require.NoError(t, err)
	assert.Equal(t, "waitingForApproval", info.Status)
	assert.True(t, info.CanBeApprovedByCurrentUser)
}
