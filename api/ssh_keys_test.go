package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSSHKeys(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/projects/id:MyProject/sshKeys")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SSHKeyList{
			SSHKey: []SSHKey{
				{Name: "deploy-key", Encrypted: false, PublicKey: "ssh-ed25519 AAAA"},
			},
		})
	})

	result, err := client.GetSSHKeys("MyProject")
	require.NoError(t, err)
	assert.Len(t, result.SSHKey, 1)
	assert.Equal(t, "deploy-key", result.SSHKey[0].Name)
}

func TestGenerateSSHKey(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/projects/id:MyProject/sshKeys/generated")
		assert.Equal(t, "my-key", r.URL.Query().Get("keyName"))
		assert.Equal(t, "ed25519", r.URL.Query().Get("keyType"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SSHKey{
			Name:      "my-key",
			PublicKey: "ssh-ed25519 GENERATED",
			Project:   &Project{ID: "MyProject"},
		})
	})

	key, err := client.GenerateSSHKey("MyProject", "my-key", "ed25519")
	require.NoError(t, err)
	assert.Equal(t, "my-key", key.Name)
	assert.Equal(t, "ssh-ed25519 GENERATED", key.PublicKey)
}
