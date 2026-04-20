package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBuildMessages(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Equal(t, "/app/messages", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("buildId"))
		assert.Equal(t, "0,-10", r.URL.Query().Get("messagesCount"))
		assert.Equal(t, "tail", r.URL.Query().Get("target"))
		assert.Equal(t, "true", r.URL.Query().Get("expandAll"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildMessagesResponse{
			Messages: []BuildMessage{
				{ID: 90, Text: "Compiling...", Level: 1, Status: 1, Timestamp: "2026-04-07T12:00:01.000+0000"},
				{ID: 100, Text: "Build finished", Level: 1, Status: 1, Timestamp: "2026-04-07T12:00:10.000+0000"},
			},
			LastMessageIndex:    100,
			FocusIndex:          100,
			LastMessageIncluded: true,
		})
	})

	resp, err := client.GetBuildMessages(t.Context(), "1", BuildMessagesOptions{
		Count:     -10,
		Tail:      true,
		ExpandAll: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, len(resp.Messages))
	assert.Equal(t, 100, resp.LastMessageIndex)
	assert.Equal(t, "Build finished", resp.Messages[1].Text)
}

func TestGetBuildMessages_head(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Equal(t, "0,5", r.URL.Query().Get("messagesCount"))
		assert.Empty(t, r.URL.Query().Get("target"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildMessagesResponse{
			Messages: []BuildMessage{
				{ID: 0, Text: "Build started", Level: 1, Status: 1},
				{ID: 1, Text: "Checkout...", Level: 1, Status: 1},
			},
			LastMessageIndex: 1,
		})
	})

	resp, err := client.GetBuildMessages(t.Context(), "1", BuildMessagesOptions{
		Count:     5,
		ExpandAll: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, len(resp.Messages))
	assert.Equal(t, "Build started", resp.Messages[0].Text)
}

func TestGetBuildMessages_withSinceID(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Equal(t, "50,-100", r.URL.Query().Get("messagesCount"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildMessagesResponse{
			Messages:         []BuildMessage{},
			LastMessageIndex: 50,
		})
	})

	_, err := client.GetBuildMessages(t.Context(), "1", BuildMessagesOptions{
		SinceID:   50,
		Count:     -100,
		Tail:      true,
		ExpandAll: true,
	})
	require.NoError(t, err)
}
