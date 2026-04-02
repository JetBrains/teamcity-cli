package auth_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthStatus(T *testing.T) {
	cmdtest.SetupMockClient(T)
	cmdtest.RunCmd(T, "auth", "status")
}

func TestAuthStatusJSON(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	got := cmdtest.CaptureOutput(T, ts.Factory, "auth", "status", "--json")
	assert.Contains(T, got, `"server"`)
	assert.Contains(T, got, `"status"`)
	assert.Contains(T, got, `"authenticated"`)
	assert.Contains(T, got, `"user"`)
}

func TestBuildAuthFallback(T *testing.T) {
	basicAuthUsed := false
	ts := cmdtest.NewTestServer(T)

	ts.Handle("GET /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if ok && user == "buildUser123" && pass == "buildPass456" {
			basicAuthUsed = true
			cmdtest.JSON(w, api.ProjectList{Count: 1, Projects: []api.Project{{ID: "Test", Name: "Test"}}})
			return
		}
		if auth := r.Header.Get("Authorization"); auth != "" {
			cmdtest.JSON(w, api.ProjectList{Count: 1, Projects: []api.Project{{ID: "Test", Name: "Test"}}})
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

	ts.Factory.ClientFunc = func() (api.ClientInterface, error) {
		buildAuth, ok := config.GetBuildAuth()
		if !ok {
			T.Fatal("Expected build auth to be available")
		}
		return api.NewClientWithBasicAuth(buildAuth.ServerURL, buildAuth.Username, buildAuth.Password), nil
	}

	cmdtest.RunCmdWithFactory(T, ts.Factory, "project", "list")
	assert.True(T, basicAuthUsed, "basic auth should have been used")
}

func TestBuildAuthFromBuildURL(T *testing.T) {
	ts := cmdtest.NewTestServer(T)

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
	ts := cmdtest.NewTestServer(T)
	var authMethod string

	ts.Handle("GET /app/rest/users/current", func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
			authMethod = "bearer"
			cmdtest.JSON(w, api.User{ID: 1, Username: "tokenUser", Name: "Token User"})
			return
		}
		if _, _, ok := r.BasicAuth(); ok {
			authMethod = "basic"
			cmdtest.JSON(w, api.User{ID: 99, Username: "buildUser", Name: "Build User"})
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})

	ts.Handle("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Server{VersionMajor: 2025, VersionMinor: 7})
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

	cmdtest.RunCmd(T, "auth", "status")
	assert.Equal(T, "bearer", authMethod, "explicit token should take precedence")
}

func TestIsBuildEnvironment(T *testing.T) {
	T.Setenv("TEAMCITY_BUILD_PROPERTIES_FILE", "/some/path")
	assert.True(T, config.IsBuildEnvironment())

	T.Setenv("TEAMCITY_BUILD_PROPERTIES_FILE", "")
	assert.False(T, config.IsBuildEnvironment())
}

func TestGuestAuthEnvVar(T *testing.T) {
	T.Setenv("TEAMCITY_GUEST", "1")
	T.Setenv("TEAMCITY_URL", "https://example.com")
	T.Setenv("TEAMCITY_TOKEN", "")
	config.ResetForTest()
	config.Init()

	assert.True(T, config.IsGuestAuth())
	assert.True(T, config.IsConfigured())
}

func TestGuestAuthEnvVarValues(T *testing.T) {
	for _, val := range []string{"1", "true", "yes"} {
		T.Run(val, func(t *testing.T) {
			t.Setenv("TEAMCITY_GUEST", val)
			t.Setenv("TEAMCITY_URL", "https://example.com")
			config.ResetForTest()
			config.Init()

			assert.True(t, config.IsGuestAuth())
		})
	}

	for _, val := range []string{"0", "false", "no", ""} {
		T.Run("not_"+val, func(t *testing.T) {
			t.Setenv("TEAMCITY_GUEST", val)
			t.Setenv("TEAMCITY_URL", "https://example.com")
			config.ResetForTest()
			config.Init()

			assert.False(t, config.IsGuestAuth())
		})
	}
}

func TestGuestAuthStatus(T *testing.T) {
	ts := cmdtest.NewTestServer(T)

	ts.Handle("GET /guestAuth/app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		assert.Empty(T, auth, "guest request should not have Authorization header")
		cmdtest.JSON(w, api.Server{VersionMajor: 2024, VersionMinor: 12, BuildNumber: "176523"})
	})

	T.Setenv("TEAMCITY_URL", ts.URL)
	T.Setenv("TEAMCITY_TOKEN", "")
	T.Setenv("TEAMCITY_GUEST", "1")
	config.ResetForTest()
	config.Init()

	cmdtest.RunCmd(T, "auth", "status")
}

func TestGuestAuthTakesPriorityOverToken(T *testing.T) {
	ts := cmdtest.NewTestServer(T)

	var usedGuestPath bool
	ts.Handle("GET /guestAuth/app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		usedGuestPath = true
		cmdtest.JSON(w, api.Server{VersionMajor: 2024, VersionMinor: 12})
	})

	T.Setenv("TEAMCITY_URL", ts.URL)
	T.Setenv("TEAMCITY_TOKEN", "some-token")
	T.Setenv("TEAMCITY_GUEST", "1")
	config.ResetForTest()
	config.Init()

	assert.True(T, config.IsGuestAuth(), "guest auth should be active")

	client := api.NewGuestClient(ts.URL)
	_, err := client.GetServer()
	require.NoError(T, err)
	assert.True(T, usedGuestPath, "guest auth should use /guestAuth/ path")
}

// setupConfigAuthStatus clears env overrides and resets config so tests
// exercise the config-based path in runAuthStatus.
func setupConfigAuthStatus(t *testing.T, ts *cmdtest.TestServer) {
	t.Helper()
	t.Setenv("TEAMCITY_URL", "")
	t.Setenv("TEAMCITY_TOKEN", "")
	t.Setenv("TEAMCITY_GUEST", "")
	t.Setenv("TEAMCITY_BUILD_PROPERTIES_FILE", "")
	t.Setenv("TC_INSECURE_SKIP_WARN", "1")
	config.ResetDSLCache()
	config.ResetForTest()

	ts.Handle("GET /app/rest/users/current", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.User{ID: 1, Username: "admin", Name: "Administrator"})
	})
	ts.Handle("GET /app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Server{VersionMajor: 2025, VersionMinor: 7, BuildNumber: "197398"})
	})
}

func TestAuthStatusMultipleServers(T *testing.T) {
	ts := cmdtest.NewTestServer(T)
	setupConfigAuthStatus(T, ts)

	ts.Handle("GET /other/app/rest/users/current", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.User{ID: 2, Username: "admin", Name: "Administrator"})
	})
	ts.Handle("GET /other/app/rest/server", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Server{VersionMajor: 2024, VersionMinor: 12, BuildNumber: "176523"})
	})

	cfg := config.Get()
	cfg.DefaultServer = ts.URL
	cfg.Servers[ts.URL] = config.ServerConfig{Token: "token-1", User: "admin"}
	cfg.Servers[ts.URL+"/other"] = config.ServerConfig{Token: "token-2", User: "admin"}

	cmdtest.RunCmd(T, "auth", "status")
}

func TestAuthStatusWithDSLHint(T *testing.T) {
	ts := cmdtest.NewTestServer(T)
	setupConfigAuthStatus(T, ts)

	tmpDir := T.TempDir()
	dslDir := filepath.Join(tmpDir, ".teamcity")
	require.NoError(T, os.MkdirAll(dslDir, 0755))
	require.NoError(T, os.WriteFile(filepath.Join(dslDir, "pom.xml"), []byte(`<?xml version="1.0"?>
<project><repositories><repository>
  <id>teamcity-server</id>
  <url>https://dsl-server.example.com/app/dsl-plugins-repository</url>
</repository></repositories></project>`), 0644))

	T.Setenv("TEAMCITY_DSL_DIR", dslDir)
	config.ResetDSLCache()

	cfg := config.Get()
	cfg.DefaultServer = ts.URL
	cfg.Servers[ts.URL] = config.ServerConfig{Token: "token-1", User: "admin"}

	// DSL points to dsl-server.example.com which is not in config —
	// should still succeed (shows authenticated server + hint)
	cmdtest.RunCmd(T, "auth", "status")
}
