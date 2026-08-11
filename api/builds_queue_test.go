package api

import (
	"encoding/json"
	"io"
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

	result, _, err := client.GetBuildQueue(QueueOptions{})
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

	_, _, err := client.GetBuildQueue(QueueOptions{BuildTypeID: "bt1"})
	require.NoError(t, err)
}

func TestRemoveFromQueue(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/app/rest/buildQueue/id:100", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"comment":"","readdIntoQueue":false}`, string(body))

		w.WriteHeader(http.StatusOK)
	})

	err := client.RemoveFromQueue("100")
	require.NoError(t, err)
}

func TestMoveQueuedBuildToTop(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/app/rest/buildQueue/order/first", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"id":"100"}`, string(body))

		w.WriteHeader(http.StatusNoContent)
	})

	err := client.MoveQueuedBuildToTop("100")
	require.NoError(t, err)
}

func TestGetQueuedBuildApprovalInfo(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/app/rest/buildQueue/id:100/approvalInfo", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApprovalInfo{Status: "waitingForApproval", CanBeApprovedByCurrentUser: true})
	})

	info, err := client.GetQueuedBuildApprovalInfo("100")
	require.NoError(t, err)
	assert.Equal(t, "waitingForApproval", info.Status)
	assert.True(t, info.CanBeApprovedByCurrentUser)
}
