package project_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionList(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	out := cmdtest.CaptureOutput(t, f, "project", "connection", "list", "--project", "TestProject")
	assert.Contains(t, out, "PROJECT_EXT_1")
	assert.Contains(t, out, "GitHub App")
}

func TestConnectionCreateGitHubApp(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	pemPath := filepath.Join(t.TempDir(), "key.pem")
	require.NoError(t, os.WriteFile(pemPath, []byte("-----BEGIN PRIVATE KEY-----\nMIICabc\n-----END PRIVATE KEY-----\n"), 0o600))

	var captured api.ProjectFeature
	ts.Handle("POST /app/rest/projects/id:TestProject/projectFeatures", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		captured.ID = "PROJECT_EXT_42"
		cmdtest.JSON(w, captured)
	})

	out := cmdtest.CaptureOutput(t, f, "project", "connection", "create", "github-app",
		"--project", "TestProject",
		"--name", "Backend",
		"--owner", "my-org",
		"--app-id", "1234567",
		"--client-id", "Iv1.abc",
		"--client-secret", "shh",
		"--private-key-file", pemPath,
	)

	assert.Contains(t, out, "Created connection")
	assert.Contains(t, out, "PROJECT_EXT_42")
	assert.Equal(t, "OAuthProvider", captured.Type)

	props := propMap(captured.Properties)
	assert.Equal(t, "GitHubApp", props["providerType"])
	assert.Equal(t, "gitHubApp", props["connectionSubtype"])
	assert.Equal(t, "Backend", props["displayName"])
	assert.Equal(t, "https://github.com/my-org", props["gitHubApp.ownerUrl"])
	assert.Equal(t, "1234567", props["gitHubApp.appId"])
	assert.Equal(t, "Iv1.abc", props["gitHubApp.clientId"])
	assert.Equal(t, "shh", props["secure:gitHubApp.clientSecret"])
	assert.Contains(t, props["secure:gitHubApp.privateKey"], "BEGIN PRIVATE KEY")
	assert.Equal(t, "false", props["useUniqueRedirect"])
}

func TestConnectionCreateGitHubAppNoOwner(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	pemPath := filepath.Join(t.TempDir(), "key.pem")
	require.NoError(t, os.WriteFile(pemPath, []byte("-----BEGIN PRIVATE KEY-----\nMIICabc\n-----END PRIVATE KEY-----\n"), 0o600))

	var captured api.ProjectFeature
	ts.Handle("POST /app/rest/projects/id:TestProject/projectFeatures", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		captured.ID = "PROJECT_EXT_43"
		cmdtest.JSON(w, captured)
	})

	out := cmdtest.CaptureOutput(t, f, "project", "connection", "create", "github-app",
		"--project", "TestProject",
		"--name", "Backend",
		"--app-id", "1234567",
		"--client-id", "Iv1.abc",
		"--client-secret", "shh",
		"--private-key-file", pemPath,
	)

	assert.Contains(t, out, "Created connection")
	props := propMap(captured.Properties)
	_, hasOwner := props["gitHubApp.ownerUrl"]
	assert.False(t, hasOwner, "ownerUrl property should be omitted when --owner is not supplied")
}

func TestConnectionCreateGitHubAppEmptyKey(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	pemPath := filepath.Join(t.TempDir(), "empty.pem")
	require.NoError(t, os.WriteFile(pemPath, []byte("   \n  \n"), 0o600))

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "private key", "project", "connection", "create", "github-app",
		"--project", "TestProject",
		"--name", "Backend",
		"--owner", "my-org",
		"--app-id", "1234567",
		"--client-id", "Iv1.abc",
		"--client-secret", "shh",
		"--private-key-file", pemPath,
	)
}

func TestConnectionCreateDocker(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	var captured api.ProjectFeature
	ts.Handle("POST /app/rest/projects/id:TestProject/projectFeatures", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		captured.ID = "PROJECT_EXT_55"
		cmdtest.JSON(w, captured)
	})

	out := cmdtest.CaptureOutput(t, f, "project", "connection", "create", "docker",
		"--project", "TestProject",
		"--name", "GHCR",
		"--url", "https://ghcr.io",
		"--username", "my-org",
		"--password", "ghp_xxx",
	)

	assert.Contains(t, out, "Created connection")
	assert.Contains(t, out, "PROJECT_EXT_55")
	assert.Contains(t, out, "service account")

	props := propMap(captured.Properties)
	assert.Equal(t, "Docker", props["providerType"])
	assert.Equal(t, "GHCR", props["displayName"])
	assert.Equal(t, "https://ghcr.io", props["repositoryUrl"])
	assert.Equal(t, "my-org", props["userName"])
	assert.Equal(t, "ghp_xxx", props["secure:userPass"])
}

func TestConnectionCreateDockerMissingURL(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "url", "project", "connection", "create", "docker",
		"--project", "TestProject",
		"--name", "GHCR",
		"--username", "my-org",
		"--password", "ghp_xxx",
	)
}

func TestConnectionDelete(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	called := false
	ts.Handle("DELETE /app/rest/projects/id:TestProject/projectFeatures/id:PROJECT_EXT_42", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	out := cmdtest.CaptureOutput(t, f, "project", "connection", "delete", "PROJECT_EXT_42",
		"--project", "TestProject", "--force")

	assert.True(t, called, "DELETE should hit the mock")
	assert.Contains(t, out, "Deleted connection")
	assert.Contains(t, out, "PROJECT_EXT_42")
}

func propMap(pl *api.PropertyList) map[string]string {
	m := map[string]string{}
	if pl == nil {
		return m
	}
	for _, p := range pl.Property {
		m[p.Name] = p.Value
	}
	return m
}
