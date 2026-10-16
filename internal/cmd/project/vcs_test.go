package project_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVcsList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "list", "--project", "TestProject")
	assert.Contains(T, out, "TestProject_Repo")
	assert.Contains(T, out, "My Repo")
	assert.Contains(T, out, "Git")
}

func TestVcsListWeb(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "vcs", "list", "--project", "TestProject", "--web")
	assert.Contains(t, out, ts.URL+"/admin/editProject.html?projectId=TestProject&tab=projectVcsRoots")
}

func TestVcsListWebValidatesLimit(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "limit", "project", "vcs", "list", "--limit", "-1", "--web")
}

func TestVcsListJSON(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "list", "--project", "TestProject", "--json")
	assert.Contains(T, out, `"id"`)
	assert.Contains(T, out, `"count"`)
}

func TestVcsListPlain(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "list", "--project", "TestProject", "--plain")
	assert.Contains(T, out, "TestProject_Repo")
	assert.Contains(T, out, "\t")
}

func TestVcsListDefaultProject(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "list")
	assert.Contains(T, out, "TestProject_Repo")
}

func TestVcsView(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "view", "TestProject_Repo")
	assert.Contains(T, out, "My Repo")
	assert.Contains(T, out, "ID: TestProject_Repo")
	assert.Contains(T, out, "Type: Git")
	assert.Contains(T, out, "Project: TestProject")
	assert.Contains(T, out, "URL: https://github.com/org/repo")
	assert.Contains(T, out, "Branch: refs/heads/main")
	assert.Contains(T, out, "Auth Method: PASSWORD")
	assert.Contains(T, out, "Password: ********")
}

func TestVcsViewJSON(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "view", "TestProject_Repo", "--json")
	assert.Contains(T, out, `"id"`)
	assert.Contains(T, out, `"properties"`)
}

func TestVcsViewNotFound(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(T, f, "No VCS root found", "project", "vcs", "view", "NonExistentVcsRoot123456")
}

func TestVcsDelete(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "delete", "TestProject_Repo", "--yes")
	assert.Contains(T, out, "Deleted VCS root TestProject_Repo")
}

func TestVcsCreateAnonymous(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "create",
		"--url", "https://github.com/org/repo.git",
		"--auth", "anonymous",
		"--project", "TestProject",
	)
	assert.Contains(T, out, "Testing connection...")
	assert.Contains(T, out, "Created VCS root")
	assert.Contains(T, out, "TestProject_NewRoot")
}

func TestVcsCreateNoTest(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	out := cmdtest.CaptureOutput(T, f, "project", "vcs", "create",
		"--project", "TestProject",
		"--url", "https://github.com/org/repo.git",
		"--auth", "anonymous",
		"--no-test",
	)
	assert.NotContains(T, out, "Testing connection")
	assert.Contains(T, out, "Created VCS root")
}

func TestVcsCreateMissingURL(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(T, f, "url", "project", "vcs", "create", "--project", "TestProject", "--auth", "anonymous")
}

func TestVcsTestReturnsServerFailure(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("GET /admin/editVcsRoot.html", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<form id="vcsSettingsForm"><input type="hidden" name="publicKey" value="key"><input type="hidden" name="prop:encrypted:secure:password" value="encrypted"></form>`)
	})
	ts.Handle("POST /admin/editVcsRoot.html", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "testConnection", r.FormValue("submitVcsRoot"))
		fmt.Fprint(w, `<response><errors><error id="failedTestConnection">repository access denied</error></errors></response>`)
	})
	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "test connection failed: repository access denied", "project", "vcs", "test", "TestProject_Repo")
}

func TestVcsStoredTokenSkipsPreflightHintForJSON(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Factory.JSONOutput = true
	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "vcs", "create", "--project", "TestProject", "--url", "https://github.com/org/repo.git", "--auth", "token", "--token-id", "tc_token_id:CID_test:-1:uuid", "--no-input")
	assert.NotContains(t, out, "Test the stored token")
	assert.NotContains(t, out, "Testing connection")
	assert.Contains(t, out, "Created VCS root")
}

func TestVcsCreateJSON(t *testing.T) {
	for _, auth := range []string{"anonymous", "token"} {
		t.Run(auth, func(t *testing.T) {
			ts := cmdtest.SetupMockClient(t)
			args := []string{"project", "vcs", "create", "--project", "TestProject", "--url", "https://github.com/org/repo.git", "--auth", auth, "--json"}
			if auth == "token" {
				args = append(args, "--token-id", "tc_token_id:CID_test:-1:uuid")
			}
			out := cmdtest.CaptureOutput(t, ts.Factory, args...)
			var root api.VcsRoot
			require.NoError(t, json.Unmarshal([]byte(out), &root))
			assert.Equal(t, "TestProject_NewRoot", root.ID)
			assert.NotContains(t, out, "Created VCS root")
			assert.NotContains(t, out, "Test the stored token")
		})
	}
}

// createdRootProps records the properties and connection of the root vcs create posts.
func createdRootProps(t *testing.T, ts *cmdtest.TestServer) (map[string]string, *string) {
	t.Helper()
	props := map[string]string{}
	var connectionID string
	ts.Handle("POST /app/rest/vcs-roots", func(w http.ResponseWriter, r *http.Request) {
		var root api.VcsRoot
		require.NoError(t, json.NewDecoder(r.Body).Decode(&root))
		for _, p := range root.Properties.Property {
			props[p.Name] = p.Value
		}
		connectionID = root.ConnectionID
		root.ID = "TestProject_NewRoot"
		cmdtest.JSON(w, root)
	})
	ts.Handle("POST /app/pipeline/repository/testConnection", func(w http.ResponseWriter, r *http.Request) {
		t.Error("a GitHub App root is created from a stored token and is not tested by connection")
	})
	return props, &connectionID
}

func TestVcsCreateGitHubAppIssuesInstallationToken(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	props, connectionID := createdRootProps(t, ts)
	ts.Handle("POST /app/oauth/githubapp/installationToken", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "TestProject", r.FormValue("projectId"))
		assert.Equal(t, "PROJECT_EXT_1", r.FormValue("connectionId"))
		assert.Equal(t, "repo", r.FormValue("repositoryName"))
		cmdtest.JSON(w, map[string]string{"tokenId": "tc_token_id:CID_x:-1:installation"})
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "vcs", "create", "--project", "TestProject",
		"--url", "https://github.com/org/repo.git", "--connection-id", "PROJECT_EXT_1", "--no-input")

	assert.Contains(t, out, "Created VCS root")
	assert.Equal(t, "ACCESS_TOKEN", props["authMethod"])
	assert.Equal(t, "tc_token_id:CID_x:-1:installation", props["tokenId"])
	assert.Equal(t, "oauth2", props["username"])
	assert.NotContains(t, props, "secure:password")
	assert.Empty(t, *connectionID, "a connectionId makes the server replace the auth with the user's token")
}

func TestVcsCreateGitHubAppReportsLoginPage(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("POST /app/oauth/githubapp/installationToken", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<html>Sign in</html>")
	})

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "may require browser sign-in",
		"project", "vcs", "create", "--project", "TestProject",
		"--url", "https://github.com/org/repo.git", "--connection-id", "PROJECT_EXT_1", "--no-input")
}

func TestVcsCreateGitHubAppNeedsAuthorization(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	ts.Handle("POST /app/oauth/githubapp/installationToken", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"status":404,"message":"No personal access token was provided via header X-TC-GitHub-PAT and no suitable value could be found in token storage either"}`)
	})

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "you have not authorized connection PROJECT_EXT_1",
		"project", "vcs", "create", "--project", "TestProject",
		"--url", "https://github.com/org/repo.git", "--connection-id", "PROJECT_EXT_1", "--no-input")
}
