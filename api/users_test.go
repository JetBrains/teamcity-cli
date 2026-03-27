package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUser(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/users/username:admin")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(User{ID: 1, Username: "admin", Name: "Administrator"})
	})

	user, err := client.GetUser("admin")
	require.NoError(t, err)
	assert.Equal(t, "admin", user.Username)
}

func TestUserExists(t *testing.T) {
	t.Parallel()

	t.Run("exists", func(t *testing.T) {
		t.Parallel()
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(User{ID: 1, Username: "admin"})
		})
		assert.True(t, client.UserExists("admin"))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"message": "not found"}}})
		})
		assert.False(t, client.UserExists("ghost"))
	})
}

func TestCreateAPIToken(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/users/current/tokens/my-token")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Token{Name: "my-token", Value: "secret-value"})
	})

	token, err := client.CreateAPIToken("my-token")
	require.NoError(t, err)
	assert.Equal(t, "my-token", token.Name)
	assert.Equal(t, "secret-value", token.Value)
}

func TestDeleteAPIToken(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/users/current/tokens/my-token")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteAPIToken("my-token")
	require.NoError(t, err)
}
