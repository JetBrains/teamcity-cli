package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"
)

func keyringMockInit() {
	gokeyring.MockInit()
}

func keyringMockInitWithError(err error) {
	gokeyring.MockInitWithError(err)
}

// Note: Tests in this file cannot use t.Parallel() because they modify
// package-level state (cfg, configPath) and environment variables.

// saveCfgState saves the current cfg state and restores it on cleanup.
// Installs a mock keyring that errors by default so existing tests that
// expect tokens in config continue to work.
func saveCfgState(t *testing.T) {
	t.Helper()
	oldCfg := cfg
	oldPath := configPath
	keyringMockInitWithError(errors.New("keyring disabled in test"))
	t.Cleanup(func() {
		cfg = oldCfg
		configPath = oldPath
	})
}

// withWorkingDir changes to dir for the duration of the test.
func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
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

func TestDetectTeamCityDir(T *testing.T) {
	T.Run("env var override", func(t *testing.T) {
		ResetDSLCache()
		tmpDir := t.TempDir()
		dslDir := filepath.Join(tmpDir, "custom-dsl")
		require.NoError(t, os.Mkdir(dslDir, 0755))
		t.Setenv(EnvDSLDir, dslDir)

		got := DetectTeamCityDir()
		assert.Equal(t, dslDir, got)
	})

	T.Run("walks up tree to find .teamcity", func(t *testing.T) {
		ResetDSLCache()
		tmpDir := t.TempDir()
		tmpDir, err := filepath.EvalSymlinks(tmpDir) // macOS /var -> /private/var
		require.NoError(t, err)
		dslDir := filepath.Join(tmpDir, DefaultDSLDirTeamCity)
		subDir := filepath.Join(tmpDir, "sub", "dir")
		require.NoError(t, os.Mkdir(dslDir, 0755))
		require.NoError(t, os.MkdirAll(subDir, 0755))

		t.Setenv(EnvDSLDir, "")
		withWorkingDir(t, subDir)

		got := DetectTeamCityDir()
		assert.Equal(t, dslDir, got)
	})

	T.Run("returns empty when not found", func(t *testing.T) {
		ResetDSLCache()
		tmpDir := t.TempDir()
		t.Setenv(EnvDSLDir, "")
		withWorkingDir(t, tmpDir)

		got := DetectTeamCityDir()
		assert.Empty(t, got)
	})
}

func TestDetectServerFromDSL(T *testing.T) {
	T.Run("extracts server URL from pom.xml", func(t *testing.T) {
		ResetDSLCache()
		tmpDir := t.TempDir()
		dslDir := filepath.Join(tmpDir, DefaultDSLDirTeamCity)
		require.NoError(t, os.Mkdir(dslDir, 0755))
		pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <repositories>
        <repository>
            <id>teamcity-server</id>
            <url>https://teamcity.example.com/app/dsl-plugins-repository</url>
        </repository>
    </repositories>
</project>`
		require.NoError(t, os.WriteFile(filepath.Join(dslDir, "pom.xml"), []byte(pomContent), 0644))

		t.Setenv(EnvDSLDir, "")
		withWorkingDir(t, tmpDir)

		got := DetectServerFromDSL()
		assert.Equal(t, "https://teamcity.example.com", got)
	})

	T.Run("returns empty when no pom.xml", func(t *testing.T) {
		ResetDSLCache()
		tmpDir := t.TempDir()
		t.Setenv(EnvDSLDir, "")
		withWorkingDir(t, tmpDir)

		got := DetectServerFromDSL()
		assert.Empty(t, got)
	})
}

func TestGetTokenNoServer(T *testing.T) {
	saveCfgState(T)
	T.Setenv(EnvServerURL, "")
	T.Setenv(EnvToken, "")
	ResetDSLCache()

	cfg = &Config{
		DefaultServer: "",
		Servers:       make(map[string]ServerConfig),
	}

	got := GetToken()
	assert.Equal(T, "", got)
}

func TestGetTokenMatchesNormalizedURL(T *testing.T) {
	saveCfgState(T)
	T.Setenv(EnvToken, "")

	cfg = &Config{
		DefaultServer: "https://tc.example.com",
		Servers: map[string]ServerConfig{
			"https://tc.example.com": {Token: "config-token", User: "user"},
		},
	}

	tests := []struct {
		name   string
		envURL string
	}{
		{"trailing slash", "https://tc.example.com/"},
		{"no scheme", "tc.example.com"},
		{"no scheme trailing slash", "tc.example.com/"},
		{"exact match", "https://tc.example.com"},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvServerURL, tc.envURL)

			got := GetToken()
			assert.Equal(t, "config-token", got, "TEAMCITY_URL=%q should find token", tc.envURL)
		})
	}
}

func TestGetTokenUnknownServer(T *testing.T) {
	saveCfgState(T)
	T.Setenv(EnvServerURL, "https://unknown.example.com")
	T.Setenv(EnvToken, "")

	cfg = &Config{
		DefaultServer: "https://known.example.com",
		Servers: map[string]ServerConfig{
			"https://known.example.com": {Token: "known-token", User: "user"},
		},
	}

	got := GetToken()
	assert.Equal(T, "", got)
}

func TestGetCurrentUserNoServer(T *testing.T) {
	saveCfgState(T)
	T.Setenv(EnvServerURL, "")
	T.Setenv(EnvToken, "")
	ResetDSLCache()

	cfg = &Config{
		DefaultServer: "",
		Servers:       make(map[string]ServerConfig),
	}

	got := GetCurrentUser()
	assert.Equal(T, "", got)
}

func TestRemoveDefaultServer(T *testing.T) {
	saveCfgState(T)
	tmpDir := T.TempDir()
	configPath = tmpDir + "/config.yml"
	cfg = &Config{Servers: make(map[string]ServerConfig)}

	// Add two servers
	err := SetServer("https://tc1.example.com", "token1", "user1")
	require.NoError(T, err)
	err = SetServer("https://tc2.example.com", "token2", "user2")
	require.NoError(T, err)

	// Default is now tc2 (the last one set)
	assert.Equal(T, "https://tc2.example.com", cfg.DefaultServer)

	// Remove the default server
	err = RemoveServer("https://tc2.example.com")
	require.NoError(T, err)

	// A new default should have been picked from the remaining servers
	assert.Equal(T, "https://tc1.example.com", cfg.DefaultServer)
	_, ok := cfg.Servers["https://tc2.example.com"]
	assert.False(T, ok, "removed server should not exist")
}

func TestGetServerURLPriority(T *testing.T) {
	saveCfgState(T)

	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <repositories>
        <repository>
            <id>teamcity-server</id>
            <url>https://dsl-server.example.com/app/dsl-plugins-repository</url>
        </repository>
    </repositories>
</project>`

	T.Run("env > DSL > config", func(t *testing.T) {
		ResetDSLCache()
		tmpDir := t.TempDir()
		dslDir := filepath.Join(tmpDir, DefaultDSLDirTeamCity)
		require.NoError(t, os.Mkdir(dslDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dslDir, "pom.xml"), []byte(pomContent), 0644))

		withWorkingDir(t, tmpDir)
		cfg = &Config{DefaultServer: "https://config.example.com"}

		// Env var takes priority
		t.Setenv(EnvDSLDir, "")
		t.Setenv(EnvServerURL, "https://env.example.com")
		assert.Equal(t, "https://env.example.com", GetServerURL())

		// DSL takes priority over config
		t.Setenv(EnvServerURL, "")
		ResetDSLCache() // reset cache to re-detect
		assert.Equal(t, "https://dsl-server.example.com", GetServerURL())

		// Falls back to config
		require.NoError(t, os.RemoveAll(dslDir))
		ResetDSLCache() // reset cache to re-detect
		assert.Equal(t, "https://config.example.com", GetServerURL())
	})
}

func TestInitHomeDirError(T *testing.T) {
	saveCfgState(T)
	old := userHomeDirFn
	T.Cleanup(func() { userHomeDirFn = old })
	userHomeDirFn = func() (string, error) { return "", errors.New("no home") }

	err := Init()
	require.Error(T, err)
	assert.Contains(T, err.Error(), "home directory")
}

func TestInitMkdirFails(T *testing.T) {
	saveCfgState(T)
	tmpDir := T.TempDir()
	// Create a regular file where the config dir hierarchy would need to go
	blocker := filepath.Join(tmpDir, ".config")
	require.NoError(T, os.WriteFile(blocker, []byte("not a dir"), 0644))

	T.Setenv("HOME", tmpDir)
	T.Setenv("USERPROFILE", tmpDir)

	err := Init()
	require.Error(T, err)
	assert.Contains(T, err.Error(), "config directory")
}

func TestInitInvalidConfig(T *testing.T) {
	saveCfgState(T)
	viper.Reset()
	tmpDir := T.TempDir()
	T.Setenv("HOME", tmpDir)
	T.Setenv("USERPROFILE", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "tc")
	require.NoError(T, os.MkdirAll(configDir, 0700))
	require.NoError(T, os.WriteFile(filepath.Join(configDir, "config.yml"), []byte(":\x00\xff invalid yaml [[["), 0644))

	err := Init()
	require.Error(T, err)
	assert.Contains(T, err.Error(), "failed to read config")
}

func TestInitUnmarshalError(T *testing.T) {
	saveCfgState(T)
	viper.Reset()
	tmpDir := T.TempDir()
	T.Setenv("HOME", tmpDir)
	T.Setenv("USERPROFILE", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "tc")
	require.NoError(T, os.MkdirAll(configDir, 0700))
	// servers as a string instead of a map causes Unmarshal to fail
	require.NoError(T, os.WriteFile(filepath.Join(configDir, "config.yml"), []byte("servers: not-a-map\n"), 0644))

	err := Init()
	require.Error(T, err)
	assert.Contains(T, err.Error(), "failed to parse config")
}

func TestInitServersDefaulted(T *testing.T) {
	saveCfgState(T)
	viper.Reset()
	tmpDir := T.TempDir()
	T.Setenv("HOME", tmpDir)
	T.Setenv("USERPROFILE", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "tc")
	require.NoError(T, os.MkdirAll(configDir, 0700))
	require.NoError(T, os.WriteFile(filepath.Join(configDir, "config.yml"), []byte("default_server: https://tc.example.com\n"), 0644))

	err := Init()
	require.NoError(T, err)
	require.NotNil(T, cfg)
	assert.NotNil(T, cfg.Servers, "viper.SetDefault should ensure servers map is initialized")
}

func TestGetCurrentUserUnknownServer(T *testing.T) {
	saveCfgState(T)
	T.Setenv(EnvServerURL, "https://unknown.example.com")

	cfg = &Config{
		DefaultServer: "https://known.example.com",
		Servers: map[string]ServerConfig{
			"https://known.example.com": {Token: "token", User: "user"},
		},
	}

	got := GetCurrentUser()
	assert.Equal(T, "", got)
}

func TestSetServerWriteError(T *testing.T) {
	saveCfgState(T)
	configPath = "/dev/null/impossible/path/config.yml"
	cfg = &Config{Servers: make(map[string]ServerConfig)}

	err := SetServer("https://tc.example.com", "token", "user")
	require.Error(T, err)
	assert.Contains(T, err.Error(), "failed to write config")
}

func TestRemoveServerWriteError(T *testing.T) {
	saveCfgState(T)
	tmpDir := T.TempDir()
	configPath = tmpDir + "/config.yml"
	cfg = &Config{
		DefaultServer: "https://tc.example.com",
		Servers: map[string]ServerConfig{
			"https://tc.example.com": {Token: "token", User: "user"},
		},
	}
	// First write must succeed so viper knows the file
	viper.Set("default_server", cfg.DefaultServer)
	viper.Set("servers", cfg.Servers)
	require.NoError(T, viper.WriteConfigAs(configPath))

	// Now point to unwritable path
	configPath = "/dev/null/impossible/path/config.yml"

	err := RemoveServer("https://tc.example.com")
	require.Error(T, err)
	assert.Contains(T, err.Error(), "failed to write config")
}

func TestDetectDSLDirEnvNotExist(T *testing.T) {
	ResetDSLCache()
	T.Setenv(EnvDSLDir, "/nonexistent/path/that/does/not/exist")

	got := DetectTeamCityDir()
	assert.Empty(T, got)
}

func TestDetectTeamCityDirGetwdError(T *testing.T) {
	ResetDSLCache()
	T.Setenv(EnvDSLDir, "")
	old := getwdFn
	T.Cleanup(func() { getwdFn = old })
	getwdFn = func() (string, error) { return "", errors.New("getwd failed") }

	got := DetectTeamCityDir()
	assert.Empty(T, got)
}

func TestDetectServerFromDSLNoMatch(T *testing.T) {
	ResetDSLCache()
	tmpDir := T.TempDir()
	dslDir := filepath.Join(tmpDir, DefaultDSLDirTeamCity)
	require.NoError(T, os.Mkdir(dslDir, 0755))

	// pom.xml without the teamcity-server repo pattern
	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <repositories>
        <repository>
            <id>some-other-repo</id>
            <url>https://example.com/repo</url>
        </repository>
    </repositories>
</project>`
	require.NoError(T, os.WriteFile(filepath.Join(dslDir, "pom.xml"), []byte(pomContent), 0644))

	T.Setenv(EnvDSLDir, "")
	withWorkingDir(T, tmpDir)

	got := DetectServerFromDSL()
	assert.Empty(T, got)
}

func TestGetTokenPriority(T *testing.T) {
	saveCfgState(T)
	keyringMockInit()

	serverURL := "https://tc.example.com"
	require.NoError(T, keyringSet("tc:"+serverURL, "admin", "keyring-token"))

	cfg = &Config{
		DefaultServer: serverURL,
		Servers: map[string]ServerConfig{
			serverURL: {Token: "config-token", User: "admin"},
		},
	}

	T.Run("env wins over keyring", func(t *testing.T) {
		t.Setenv(EnvToken, "env-token")
		t.Setenv(EnvServerURL, serverURL)

		token, source := GetTokenWithSource()
		assert.Equal(t, "env-token", token)
		assert.Equal(t, "env", source)
	})

	T.Run("keyring wins over config", func(t *testing.T) {
		t.Setenv(EnvToken, "")
		t.Setenv(EnvServerURL, serverURL)

		token, source := GetTokenWithSource()
		assert.Equal(t, "keyring-token", token)
		assert.Equal(t, "keyring", source)
	})

	T.Run("config used when keyring empty", func(t *testing.T) {
		t.Setenv(EnvToken, "")
		t.Setenv(EnvServerURL, serverURL)
		require.NoError(t, keyringDelete("tc:"+serverURL, "admin"))

		token, source := GetTokenWithSource()
		assert.Equal(t, "config-token", token)
		assert.Equal(t, "config", source)
	})
}

func TestSetServerWithKeyring(T *testing.T) {
	saveCfgState(T)
	keyringMockInit()
	tmpDir := T.TempDir()
	configPath = tmpDir + "/config.yml"
	cfg = &Config{Servers: make(map[string]ServerConfig)}

	insecure, err := SetServerWithKeyring("https://tc.example.com", "my-token", "admin", false)
	require.NoError(T, err)
	assert.False(T, insecure)

	// Token in keyring, not in config
	assert.Empty(T, cfg.Servers["https://tc.example.com"].Token)
	assert.Equal(T, "admin", cfg.Servers["https://tc.example.com"].User)
	val, err := keyringGet("tc:https://tc.example.com", "admin")
	require.NoError(T, err)
	assert.Equal(T, "my-token", val)
}

func TestSetServerKeyringFallback(T *testing.T) {
	saveCfgState(T)
	tmpDir := T.TempDir()
	configPath = tmpDir + "/config.yml"
	cfg = &Config{Servers: make(map[string]ServerConfig)}

	insecure, err := SetServerWithKeyring("https://tc.example.com", "my-token", "admin", false)
	require.NoError(T, err)
	assert.True(T, insecure)

	assert.Equal(T, "my-token", cfg.Servers["https://tc.example.com"].Token)
}

func TestRemoveServerCleansKeyring(T *testing.T) {
	saveCfgState(T)
	keyringMockInit()
	tmpDir := T.TempDir()
	configPath = tmpDir + "/config.yml"
	cfg = &Config{Servers: make(map[string]ServerConfig)}

	_, err := SetServerWithKeyring("https://tc.example.com", "my-token", "admin", false)
	require.NoError(T, err)

	err = RemoveServer("https://tc.example.com")
	require.NoError(T, err)

	_, ok := cfg.Servers["https://tc.example.com"]
	assert.False(T, ok)
	_, err = keyringGet("tc:https://tc.example.com", "admin")
	assert.ErrorIs(T, err, errKeyringNotFound)
}

func TestIsReadOnly(T *testing.T) {
	saveCfgState(T)

	T.Run("env var", func(t *testing.T) {
		for _, env := range []string{"1", "true", "yes"} {
			t.Setenv(EnvReadOnly, env)
			assert.True(t, IsReadOnly(), "TEAMCITY_RO=%q should be read-only", env)
		}
		for _, env := range []string{"", "0", "false"} {
			t.Setenv(EnvReadOnly, env)
			assert.False(t, IsReadOnly(), "TEAMCITY_RO=%q should not be read-only", env)
		}
	})

	T.Run("config file", func(t *testing.T) {
		t.Setenv(EnvReadOnly, "")
		t.Setenv(EnvServerURL, "")
		cfg = &Config{
			DefaultServer: "https://tc.example.com",
			Servers: map[string]ServerConfig{
				"https://tc.example.com": {Token: "token", User: "user", RO: true},
			},
		}
		assert.True(t, IsReadOnly())

		cfg.Servers["https://tc.example.com"] = ServerConfig{Token: "token", User: "user", RO: false}
		assert.False(t, IsReadOnly())
	})
}
