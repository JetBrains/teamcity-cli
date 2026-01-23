package config

import (
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
