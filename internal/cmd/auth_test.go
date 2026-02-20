package cmd_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthStatus(T *testing.T) {
	setupMockClient(T)
	runCmd(T, "auth", "status")
}

func TestBuildAuthFallback(T *testing.T) {
	basicAuthUsed := false
	ts := NewTestServer(T)

	ts.Handle("GET /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if ok && user == "buildUser123" && pass == "buildPass456" {
			basicAuthUsed = true
			JSON(w, api.ProjectList{Count: 1, Projects: []api.Project{{ID: "Test", Name: "Test"}}})
			return
		}
		if auth := r.Header.Get("Authorization"); auth != "" {
			JSON(w, api.ProjectList{Count: 1, Projects: []api.Project{{ID: "Test", Name: "Test"}}})
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})

	tmpDir := T.TempDir()
	propsFile := filepath.Join(tmpDir, "build.properties")
	propsContent := `teamcity.auth.userId=buildUser123
teamcity.auth.password=buildPass456
teamcity.serverUrl=` + ts.URL + "\n"

	err := os.WriteFile(propsFile, []byte(propsContent), 0600)
	require.NoError(T, err)

	T.Setenv("TEAMCITY_BUILD_PROPERTIES_FILE", propsFile)
	T.Setenv("BUILD_URL", ts.URL+"/viewLog.html?buildId=12345")

	original := cmd.GetClientFunc
	cmd.GetClientFunc = func() (api.ClientInterface, error) {
		buildAuth, ok := config.GetBuildAuth()
		if !ok {
			T.Fatal("Expected build auth to be available")
		}
		return api.NewClientWithBasicAuth(buildAuth.ServerURL, buildAuth.Username, buildAuth.Password), nil
	}
	T.Cleanup(func() {
		cmd.GetClientFunc = original
	})

	runCmd(T, "project", "list")
	assert.True(T, basicAuthUsed, "basic auth should have been used")
}

func TestBuildAuthFromBuildURL(T *testing.T) {
	ts := NewTestServer(T)

	T.Setenv("TEAMCITY_URL", "")
	T.Setenv("TEAMCITY_TOKEN", "")

	tmpDir := T.TempDir()
	propsFile := filepath.Join(tmpDir, "build.properties")
	propsContent := `teamcity.auth.userId=buildUser
teamcity.auth.password=buildPass
`
	err := os.WriteFile(propsFile, []byte(propsContent), 0600)
	require.NoError(T, err)

	T.Setenv("TEAMCITY_BUILD_PROPERTIES_FILE", propsFile)
	T.Setenv("BUILD_URL", ts.URL+"/viewLog.html?buildId=12345&buildTypeId=Project_Build")

	buildAuth, ok := config.GetBuildAuth()
	require.True(T, ok)
	assert.Equal(T, ts.URL, buildAuth.ServerURL)
	assert.Equal(T, "buildUser", buildAuth.Username)
	assert.Equal(T, "buildPass", buildAuth.Password)
}

func TestExplicitAuthTakesPrecedenceOverBuildAuth(T *testing.T) {
	ts := NewTestServer(T)
	var authMethod string

	ts.Handle("GET /app/rest/users/current", func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
			authMethod = "bearer"
			JSON(w, api.User{ID: 1, Username: "tokenUser", Name: "Token User"})
			return
		}
		if _, _, ok := r.BasicAuth(); ok {
			authMethod = "basic"
			JSON(w, api.User{ID: 99, Username: "buildUser", Name: "Build User"})
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})

	ts.Handle("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, api.Server{VersionMajor: 2025, VersionMinor: 7})
	})

	T.Setenv("TEAMCITY_URL", ts.URL)
	T.Setenv("TEAMCITY_TOKEN", "explicit-token")

	tmpDir := T.TempDir()
	propsFile := filepath.Join(tmpDir, "build.properties")
	propsContent := `teamcity.auth.userId=buildUser
teamcity.auth.password=buildPass
teamcity.serverUrl=` + ts.URL + "\n"
	err := os.WriteFile(propsFile, []byte(propsContent), 0600)
	require.NoError(T, err)

	T.Setenv("TEAMCITY_BUILD_PROPERTIES_FILE", propsFile)
	T.Setenv("BUILD_URL", ts.URL+"/viewLog.html?buildId=123")

	config.Init()

	original := cmd.GetClientFunc
	cmd.GetClientFunc = func() (api.ClientInterface, error) {
		serverURL := config.GetServerURL()
		token := config.GetToken()
		if serverURL != "" && token != "" {
			return api.NewClient(serverURL, token), nil
		}
		buildAuth, ok := config.GetBuildAuth()
		if ok {
			return api.NewClientWithBasicAuth(buildAuth.ServerURL, buildAuth.Username, buildAuth.Password), nil
		}
		T.Fatal("No auth available")
		return nil, nil
	}
	T.Cleanup(func() {
		cmd.GetClientFunc = original
	})

	runCmd(T, "auth", "status")
	assert.Equal(T, "bearer", authMethod, "explicit token should take precedence")
}

func TestIsBuildEnvironment(T *testing.T) {
	T.Setenv("TEAMCITY_BUILD_PROPERTIES_FILE", "/some/path")
	assert.True(T, config.IsBuildEnvironment())

	T.Setenv("TEAMCITY_BUILD_PROPERTIES_FILE", "")
	assert.False(T, config.IsBuildEnvironment())
}
