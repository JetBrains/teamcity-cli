package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: Tests in this file cannot use t.Parallel() because they modify
// package-level state (cfg, configPath) and environment variables.

// saveCfgState saves the current cfg state and returns a cleanup function.
func saveCfgState(t *testing.T) {
	t.Helper()
	oldCfg := cfg
	oldPath := configPath
	t.Cleanup(func() {
		cfg = oldCfg
		configPath = oldPath
	})
}

func TestGetServerURLFromEnv(T *testing.T) {
	want := "https://teamcity.example.com"
	T.Setenv(EnvServerURL, want)

	got := GetServerURL()
	assert.Equal(T, want, got)
}

func TestGetTokenFromEnv(T *testing.T) {
	want := "test-token-123"
	T.Setenv(EnvToken, want)

	got := GetToken()
	assert.Equal(T, want, got)
}

func TestGet(T *testing.T) {
	saveCfgState(T)
	cfg = nil

	got := Get()
	require.NotNil(T, got)
	assert.NotNil(T, got.Servers)
}

func TestIsConfigured(T *testing.T) {
	saveCfgState(T)

	tests := []struct {
		name      string
		serverURL string
		token     string
		cfg       *Config
		want      bool
	}{
		{
			name:      "configured via env vars",
			serverURL: "https://teamcity.example.com",
			token:     "test-token",
			cfg:       nil,
			want:      true,
		},
		{
			name:      "not configured - empty env and config",
			serverURL: "",
			token:     "",
			cfg:       &Config{Servers: make(map[string]ServerConfig)},
			want:      false,
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvServerURL, tc.serverURL)
			t.Setenv(EnvToken, tc.token)
			cfg = tc.cfg

			got := IsConfigured()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetCurrentUser(T *testing.T) {
	saveCfgState(T)

	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{
			name: "returns user from config",
			cfg: &Config{
				DefaultServer: "https://tc.example.com",
				Servers: map[string]ServerConfig{
					"https://tc.example.com": {
						Token: "token",
						User:  "testuser",
					},
				},
			},
			want: "testuser",
		},
		{
			name: "returns empty when no default server",
			cfg: &Config{
				DefaultServer: "",
				Servers:       make(map[string]ServerConfig),
			},
			want: "",
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvServerURL, "")
			cfg = tc.cfg

			got := GetCurrentUser()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestConfigPath(T *testing.T) {
	saveCfgState(T)

	want := "/test/path/config.yml"
	configPath = want

	got := ConfigPath()
	assert.Equal(T, want, got)
}

func TestGetTokenFromConfig(T *testing.T) {
	saveCfgState(T)
	T.Setenv(EnvServerURL, "")
	T.Setenv(EnvToken, "")

	cfg = &Config{
		DefaultServer: "https://tc.example.com",
		Servers: map[string]ServerConfig{
			"https://tc.example.com": {
				Token: "config-token",
				User:  "testuser",
			},
		},
	}

	want := "config-token"
	got := GetToken()
	assert.Equal(T, want, got)
}

func TestSetAndRemoveServer(T *testing.T) {
	saveCfgState(T)
	tmpDir := T.TempDir()
	configPath = tmpDir + "/config.yml"
	cfg = &Config{Servers: make(map[string]ServerConfig)}

	// Test SetServer - first server becomes default
	err := SetServer("https://tc1.example.com", "token1", "user1")
	require.NoError(T, err)
	assert.Equal(T, "https://tc1.example.com", cfg.DefaultServer)
	assert.Equal(T, "token1", cfg.Servers["https://tc1.example.com"].Token)

	// Add second server
	err = SetServer("https://tc2.example.com", "token2", "user2")
	require.NoError(T, err)

	// Test RemoveServer (non-default)
	err = RemoveServer("https://tc1.example.com")
	require.NoError(T, err)
	_, ok := cfg.Servers["https://tc1.example.com"]
	assert.False(T, ok, "server should have been removed")

	// Test RemoveServer (last remaining server)
	err = RemoveServer("https://tc2.example.com")
	require.NoError(T, err)
	assert.Equal(T, 0, len(cfg.Servers))
}

func TestInit(T *testing.T) {
	saveCfgState(T)
	tmpDir := T.TempDir()
	T.Setenv("HOME", tmpDir)
	T.Setenv("USERPROFILE", tmpDir) // Required for Windows

	cfg = nil
	configPath = ""

	err := Init()
	require.NoError(T, err)

	want := filepath.Join(tmpDir, ".config", "tc", "config.yml")
	assert.Equal(T, want, configPath)
	require.NotNil(T, cfg)
}

func TestSetUserForServer(T *testing.T) {
	saveCfgState(T)

	T.Run("existing server", func(t *testing.T) {
		cfg = &Config{
			DefaultServer: "https://tc.example.com",
			Servers: map[string]ServerConfig{
				"https://tc.example.com": {Token: "token", User: ""},
			},
		}
		SetUserForServer("https://tc.example.com", "newuser")

		got := cfg.Servers["https://tc.example.com"].User
		assert.Equal(t, "newuser", got)
	})

	T.Run("new server entry", func(t *testing.T) {
		cfg = &Config{
			DefaultServer: "https://tc.example.com",
			Servers: map[string]ServerConfig{
				"https://tc.example.com": {Token: "token", User: "user"},
			},
		}
		SetUserForServer("https://other.example.com", "newuser")

		// Original server should be unchanged
		assert.Equal(t, "user", cfg.Servers["https://tc.example.com"].User)
		// New server should be created
		assert.Equal(t, "newuser", cfg.Servers["https://other.example.com"].User)
	})

	T.Run("nil config is no-op", func(t *testing.T) {
		cfg = nil
		// Should not panic
		SetUserForServer("https://tc.example.com", "user")
	})

	T.Run("nil servers map is no-op", func(t *testing.T) {
		cfg = &Config{DefaultServer: "https://tc.example.com", Servers: nil}
		// Should not panic
		SetUserForServer("https://tc.example.com", "user")
	})
}

func TestReadPropertiesFile(T *testing.T) {
	T.Run("basic properties", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `# Comment line
teamcity.auth.userId=build_user_123
teamcity.auth.password=secret_password
teamcity.serverUrl=https://teamcity.example.com
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		props, err := readPropertiesFile(propsFile)
		require.NoError(t, err)

		assert.Equal(t, "build_user_123", props["teamcity.auth.userId"])
		assert.Equal(t, "secret_password", props["teamcity.auth.password"])
		assert.Equal(t, "https://teamcity.example.com", props["teamcity.serverUrl"])
	})

	T.Run("colon separator", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `key1:value1
key2=value2
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		props, err := readPropertiesFile(propsFile)
		require.NoError(t, err)

		assert.Equal(t, "value1", props["key1"])
		assert.Equal(t, "value2", props["key2"])
	})

	T.Run("escaped values", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `path=C\\:\\Users\\test
multiline=line1\nline2
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		props, err := readPropertiesFile(propsFile)
		require.NoError(t, err)

		// \\\\ in Go string literal is \\ in file, which unescapes to \
		// So C\\:\Users\\test in file becomes C\:\Users\test
		assert.Equal(t, "C\\:\\Users\\test", props["path"])
		// \n in file becomes actual newline
		assert.Equal(t, "line1\nline2", props["multiline"])
	})

	T.Run("file not found", func(t *testing.T) {
		_, err := readPropertiesFile("/nonexistent/file.properties")
		assert.Error(t, err)
	})
}

func TestGetBuildAuth(T *testing.T) {
	saveCfgState(T)

	T.Run("no env var set", func(t *testing.T) {
		t.Setenv(EnvBuildPropertiesFile, "")

		auth, err := GetBuildAuth()
		assert.Nil(t, auth)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running in a TeamCity build environment")
	})

	T.Run("valid properties file", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `teamcity.auth.userId=build_user
teamcity.auth.password=build_pass
teamcity.serverUrl=https://tc.example.com/
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		t.Setenv(EnvBuildPropertiesFile, propsFile)
		t.Setenv(EnvServerURL, "")

		auth, err := GetBuildAuth()
		require.NoError(t, err)
		require.NotNil(t, auth)

		assert.Equal(t, "build_user", auth.UserID)
		assert.Equal(t, "build_pass", auth.Password)
		assert.Equal(t, "https://tc.example.com", auth.ServerURL) // trailing slash should be trimmed
	})

	T.Run("missing credentials in file", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `teamcity.serverUrl=https://tc.example.com
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		t.Setenv(EnvBuildPropertiesFile, propsFile)

		auth, err := GetBuildAuth()
		assert.Nil(t, auth)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "build auth credentials not found")
	})

	T.Run("fallback to TEAMCITY_URL", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `teamcity.auth.userId=build_user
teamcity.auth.password=build_pass
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		t.Setenv(EnvBuildPropertiesFile, propsFile)
		t.Setenv(EnvServerURL, "https://fallback.example.com")

		auth, err := GetBuildAuth()
		require.NoError(t, err)
		require.NotNil(t, auth)

		assert.Equal(t, "https://fallback.example.com", auth.ServerURL)
	})

	T.Run("missing server URL", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `teamcity.auth.userId=build_user
teamcity.auth.password=build_pass
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		t.Setenv(EnvBuildPropertiesFile, propsFile)
		t.Setenv(EnvServerURL, "")
		cfg = &Config{Servers: make(map[string]ServerConfig)}

		auth, err := GetBuildAuth()
		assert.Nil(t, auth)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TeamCity server URL not found")
	})

	T.Run("nonexistent properties file", func(t *testing.T) {
		t.Setenv(EnvBuildPropertiesFile, "/nonexistent/path/build.properties")

		auth, err := GetBuildAuth()
		assert.Nil(t, auth)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read build properties file")
	})
}
