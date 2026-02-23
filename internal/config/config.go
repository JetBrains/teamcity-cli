package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

const (
	EnvServerURL = "TEAMCITY_URL"
	EnvToken     = "TEAMCITY_TOKEN"
	EnvGuestAuth = "TEAMCITY_GUEST"
	EnvReadOnly  = "TEAMCITY_RO"
	EnvDSLDir    = "TEAMCITY_DSL_DIR"

	DefaultDSLDirTeamCity = ".teamcity"
	DefaultDSLDirTC       = ".tc"

	dslPluginsRepoSuffix = "/app/dsl-plugins-repository"
)

type ServerConfig struct {
	Token string `mapstructure:"token"`
	User  string `mapstructure:"user"`
	Guest bool   `mapstructure:"guest,omitempty"`
}

type Config struct {
	DefaultServer string                  `mapstructure:"default_server"`
	Servers       map[string]ServerConfig `mapstructure:"servers"`
	Aliases       map[string]string       `mapstructure:"aliases"`
}

var (
	cfg        *Config
	configPath string

	// injectable for testing
	userHomeDirFn = os.UserHomeDir
	getwdFn       = os.Getwd

	// cached DSL detection results
	dslDirOnce    sync.Once
	dslDirCached  string
	dslServerOnce sync.Once
	dslServerURL  string
)

func Init() error {
	home, err := userHomeDirFn()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".config", "tc")
	configPath = filepath.Join(configDir, "config.yml")

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	viper.SetDefault("servers", map[string]ServerConfig{})

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to read config: %w", err)
			}
		}
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	return nil
}

// Get returns the current config
func Get() *Config {
	if cfg == nil {
		cfg = &Config{
			Servers: make(map[string]ServerConfig),
		}
	}
	return cfg
}

// NormalizeURL trims trailing slashes and ensures an http(s) scheme prefix.
//
//goland:noinspection HttpUrlsUsage
func NormalizeURL(u string) string {
	u = strings.TrimSuffix(u, "/")
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	return u
}

func keyringService(serverURL string) string {
	return "tc:" + serverURL
}

func GetServerURL() string {
	if url := os.Getenv(EnvServerURL); url != "" {
		return NormalizeURL(url)
	}

	if url := DetectServerFromDSL(); url != "" {
		return url
	}

	return cfg.DefaultServer
}

func GetToken() string {
	token, _ := GetTokenWithSource()
	return token
}

func GetTokenWithSource() (token, source string) {
	if token := os.Getenv(EnvToken); token != "" {
		return token, "env"
	}

	serverURL := GetServerURL()
	if serverURL == "" {
		return "", ""
	}

	server, ok := cfg.Servers[serverURL]
	if ok && server.User != "" {
		if t, err := keyringGet(keyringService(serverURL), server.User); err == nil && t != "" {
			return t, "keyring"
		}
	}

	if ok && server.Token != "" {
		return server.Token, "config"
	}
	return "", ""
}

// GetCurrentUser returns the current user from config
func GetCurrentUser() string {
	serverURL := GetServerURL()
	if serverURL == "" {
		return ""
	}

	if server, ok := cfg.Servers[serverURL]; ok {
		return server.User
	}
	return ""
}

func SetServer(serverURL, token, user string) error {
	_, err := SetServerWithKeyring(serverURL, token, user, false)
	return err
}

func SetServerWithKeyring(serverURL, token, user string, insecureStorage bool) (insecureFallback bool, err error) {
	cfg.DefaultServer = serverURL

	if !insecureStorage {
		if krErr := keyringSet(keyringService(serverURL), user, token); krErr == nil {
			cfg.Servers[serverURL] = ServerConfig{User: user}
			return false, writeConfig()
		}
	}

	cfg.Servers[serverURL] = ServerConfig{Token: token, User: user}
	return true, writeConfig()
}

func writeConfig() error {
	viper.Set("default_server", cfg.DefaultServer)
	viper.Set("servers", cfg.Servers)
	viper.Set("aliases", cfg.Aliases)

	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := os.Chmod(configPath, 0600); err != nil {
		return fmt.Errorf("failed to set config permissions: %w", err)
	}
	return nil
}

func RemoveServer(serverURL string) error {
	if server, ok := cfg.Servers[serverURL]; ok && server.User != "" {
		_ = keyringDelete(keyringService(serverURL), server.User)
	}

	delete(cfg.Servers, serverURL)

	if cfg.DefaultServer == serverURL {
		cfg.DefaultServer = ""
		for url := range cfg.Servers {
			cfg.DefaultServer = url
			break
		}
	}

	return writeConfig()
}

func ConfigPath() string {
	return configPath
}

// IsGuestAuth returns true if guest authentication is enabled via env var or server config
func IsGuestAuth() bool {
	if v := os.Getenv(EnvGuestAuth); v == "1" || v == "true" || v == "yes" {
		return true
	}
	serverURL := GetServerURL()
	if serverURL == "" || cfg == nil {
		return false
	}
	if server, ok := cfg.Servers[serverURL]; ok {
		return server.Guest
	}
	return false
}

// IsReadOnly returns true if read-only mode is enabled via env var.
// When enabled, all non-GET API requests are blocked.
func IsReadOnly() bool {
	v := os.Getenv(EnvReadOnly)
	return v == "1" || v == "true" || v == "yes"
}

// SetGuestServer saves a server with guest auth enabled and no token
func SetGuestServer(serverURL string) error {
	cfg.DefaultServer = serverURL
	cfg.Servers[serverURL] = ServerConfig{Guest: true}
	return writeConfig()
}

// IsConfigured returns true if server URL and token are set, or guest auth is active
func IsConfigured() bool {
	if IsGuestAuth() && GetServerURL() != "" {
		return true
	}
	return GetServerURL() != "" && GetToken() != ""
}

func DetectTeamCityDir() string {
	dslDirOnce.Do(func() {
		dslDirCached = detectTeamCityDirUncached()
	})
	return dslDirCached
}

func detectTeamCityDirUncached() string {
	if envDir := os.Getenv(EnvDSLDir); envDir != "" {
		if abs, err := filepath.Abs(envDir); err == nil {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				return abs
			}
		}
		return ""
	}

	cwd, err := getwdFn()
	if err != nil {
		return ""
	}

	dir := cwd
	for {
		for _, name := range []string{DefaultDSLDirTeamCity, DefaultDSLDirTC} {
			candidate := filepath.Join(dir, name)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

var teamcityServerRepoRegex = regexp.MustCompile(`<id>teamcity-server</id>\s*<url>([^<]+)</url>`)

func DetectServerFromDSL() string {
	dslServerOnce.Do(func() {
		dslServerURL = detectServerFromDSLUncached()
	})
	return dslServerURL
}

func detectServerFromDSLUncached() string {
	dslDir := DetectTeamCityDir()
	if dslDir == "" {
		return ""
	}

	pomPath := filepath.Join(dslDir, "pom.xml")
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return ""
	}

	matches := teamcityServerRepoRegex.FindSubmatch(data)
	if len(matches) < 2 {
		return ""
	}

	repoURL := strings.TrimSpace(string(matches[1]))
	serverURL := strings.TrimSuffix(repoURL, "/")
	serverURL = strings.TrimSuffix(serverURL, dslPluginsRepoSuffix)
	return strings.TrimSuffix(serverURL, "/")
}

// ResetDSLCache resets the cached DSL detection results. Used by tests.
func ResetDSLCache() {
	dslDirOnce = sync.Once{}
	dslDirCached = ""
	dslServerOnce = sync.Once{}
	dslServerURL = ""
}

// SetUserForServer sets the user for a server URL in memory (does not persist to disk).
// This is useful for tests that need to set the user without modifying the config file.
func SetUserForServer(serverURL, user string) {
	if cfg == nil {
		cfg = &Config{
			Servers: make(map[string]ServerConfig),
		}
	}
	if cfg.Servers == nil {
		cfg.Servers = make(map[string]ServerConfig)
	}

	server := cfg.Servers[serverURL]
	server.User = user
	cfg.Servers[serverURL] = server
}

func SetConfigPathForTest(path string) {
	configPath = path
}

func ResetForTest() {
	cfg = &Config{
		Servers: make(map[string]ServerConfig),
		Aliases: make(map[string]string),
	}
}
