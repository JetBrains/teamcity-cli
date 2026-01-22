package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetServerURLFromEnv(t *testing.T) {
	// Save original env
	origURL := os.Getenv(EnvServerURL)
	defer os.Setenv(EnvServerURL, origURL)

	testURL := "https://teamcity.example.com"
	os.Setenv(EnvServerURL, testURL)

	result := GetServerURL()
	if result != testURL {
		t.Errorf("GetServerURL() = %q, want %q", result, testURL)
	}
}

func TestGetTokenFromEnv(t *testing.T) {
	// Save original env
	origToken := os.Getenv(EnvToken)
	defer os.Setenv(EnvToken, origToken)

	testToken := "test-token-123"
	os.Setenv(EnvToken, testToken)

	result := GetToken()
	if result != testToken {
		t.Errorf("GetToken() = %q, want %q", result, testToken)
	}
}

func TestGet(t *testing.T) {
	// Reset cfg to ensure fresh state
	cfg = nil

	result := Get()
	if result == nil {
		t.Error("Get() returned nil")
	}
	if result.Servers == nil {
		t.Error("Get().Servers is nil")
	}
}

func TestIsConfigured(t *testing.T) {
	// Save original env
	origURL := os.Getenv(EnvServerURL)
	origToken := os.Getenv(EnvToken)
	defer func() {
		os.Setenv(EnvServerURL, origURL)
		os.Setenv(EnvToken, origToken)
	}()

	// Test when configured
	os.Setenv(EnvServerURL, "https://teamcity.example.com")
	os.Setenv(EnvToken, "test-token")
	if !IsConfigured() {
		t.Error("IsConfigured() = false when both URL and token set")
	}

	// Test when not configured
	os.Setenv(EnvServerURL, "")
	os.Setenv(EnvToken, "")
	cfg = &Config{Servers: make(map[string]ServerConfig)}
	if IsConfigured() {
		t.Error("IsConfigured() = true when URL and token empty")
	}
}

func TestGetCurrentUser(t *testing.T) {
	// Save and clear env
	origURL := os.Getenv(EnvServerURL)
	defer os.Setenv(EnvServerURL, origURL)
	os.Setenv(EnvServerURL, "")

	// Set up config with a server
	cfg = &Config{
		DefaultServer: "https://tc.example.com",
		Servers: map[string]ServerConfig{
			"https://tc.example.com": {
				Token: "token",
				User:  "testuser",
			},
		},
	}

	result := GetCurrentUser()
	if result != "testuser" {
		t.Errorf("GetCurrentUser() = %q, want %q", result, "testuser")
	}
}

func TestGetCurrentUserEmpty(t *testing.T) {
	// Save and clear env
	origURL := os.Getenv(EnvServerURL)
	defer os.Setenv(EnvServerURL, origURL)
	os.Setenv(EnvServerURL, "")

	// Empty config
	cfg = &Config{
		DefaultServer: "",
		Servers:       make(map[string]ServerConfig),
	}

	result := GetCurrentUser()
	if result != "" {
		t.Errorf("GetCurrentUser() = %q, want empty string", result)
	}
}

func TestConfigPath(t *testing.T) {
	// After init, configPath should be set
	// We don't want to actually call Init in unit tests as it writes to filesystem
	// So we just verify ConfigPath returns whatever configPath is set to
	configPath = "/test/path/config.yml"
	result := ConfigPath()
	if result != "/test/path/config.yml" {
		t.Errorf("ConfigPath() = %q, want %q", result, "/test/path/config.yml")
	}
}

func TestGetTokenFromConfig(t *testing.T) {
	// Save and clear env
	origURL := os.Getenv(EnvServerURL)
	origToken := os.Getenv(EnvToken)
	defer func() {
		os.Setenv(EnvServerURL, origURL)
		os.Setenv(EnvToken, origToken)
	}()

	// Clear env vars
	os.Setenv(EnvServerURL, "")
	os.Setenv(EnvToken, "")

	// Set up config with a server
	cfg = &Config{
		DefaultServer: "https://tc.example.com",
		Servers: map[string]ServerConfig{
			"https://tc.example.com": {
				Token: "config-token",
				User:  "testuser",
			},
		},
	}

	result := GetToken()
	if result != "config-token" {
		t.Errorf("GetToken() = %q, want %q", result, "config-token")
	}
}

func TestSetAndRemoveServer(t *testing.T) {
	// Use temp dir for config
	tmpDir := t.TempDir()
	oldPath := configPath
	configPath = tmpDir + "/config.yml"
	defer func() { configPath = oldPath }()

	// Initialize fresh config
	cfg = &Config{Servers: make(map[string]ServerConfig)}

	// Test SetServer
	err := SetServer("https://tc1.example.com", "token1", "user1")
	if err != nil {
		t.Fatalf("SetServer() error = %v", err)
	}
	if cfg.DefaultServer != "https://tc1.example.com" {
		t.Errorf("DefaultServer = %q, want %q", cfg.DefaultServer, "https://tc1.example.com")
	}
	if cfg.Servers["https://tc1.example.com"].Token != "token1" {
		t.Error("Server token not set correctly")
	}

	// Add second server
	err = SetServer("https://tc2.example.com", "token2", "user2")
	if err != nil {
		t.Fatalf("SetServer() for second server error = %v", err)
	}

	// Test RemoveServer (non-default)
	err = RemoveServer("https://tc1.example.com")
	if err != nil {
		t.Fatalf("RemoveServer() error = %v", err)
	}
	if _, ok := cfg.Servers["https://tc1.example.com"]; ok {
		t.Error("Server should have been removed")
	}

	// Test RemoveServer (default - should pick another)
	err = RemoveServer("https://tc2.example.com")
	if err != nil {
		t.Fatalf("RemoveServer() error = %v", err)
	}
	if len(cfg.Servers) != 0 {
		t.Error("All servers should be removed")
	}
}

func TestInit(t *testing.T) {
	// Override HOME to use temp dir
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Reset package state
	cfg = nil
	configPath = ""

	err := Init()
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, ".config", "tc", "config.yml")
	if configPath != expectedPath {
		t.Errorf("configPath = %q, want %q", configPath, expectedPath)
	}

	if cfg == nil {
		t.Error("cfg should not be nil after Init")
	}
}
