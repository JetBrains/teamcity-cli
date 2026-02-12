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
	EnvDSLDir    = "TEAMCITY_DSL_DIR"

	DefaultDSLDirTeamCity = ".teamcity"
	DefaultDSLDirTC       = ".tc"

	dslPluginsRepoSuffix = "/app/dsl-plugins-repository"
)

type ServerConfig struct {
	Token string `mapstructure:"token"`
	User  string `mapstructure:"user"`
}

type Config struct {
	DefaultServer string                  `mapstructure:"default_server"`
	Servers       map[string]ServerConfig `mapstructure:"servers"`
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

//goland:noinspection HttpUrlsUsage
func normalizeURL(u string) string {
	u = strings.TrimSuffix(u, "/")
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	return u
}

func GetServerURL() string {
	if url := os.Getenv(EnvServerURL); url != "" {
		return normalizeURL(url)
	}

	if url := DetectServerFromDSL(); url != "" {
		return url
	}

	return cfg.DefaultServer
}

func GetToken() string {
	if token := os.Getenv(EnvToken); token != "" {
		return token
	}

	serverURL := GetServerURL()
	if serverURL == "" {
		return ""
	}

	if server, ok := cfg.Servers[serverURL]; ok {
		return server.Token
	}
	return ""
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
	cfg.DefaultServer = serverURL
	cfg.Servers[serverURL] = ServerConfig{
		Token: token,
		User:  user,
	}

	viper.Set("default_server", serverURL)
	viper.Set("servers", cfg.Servers)

	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func RemoveServer(serverURL string) error {
	delete(cfg.Servers, serverURL)

	if cfg.DefaultServer == serverURL {
		cfg.DefaultServer = ""
		for url := range cfg.Servers {
			cfg.DefaultServer = url
			break
		}
	}

	viper.Set("default_server", cfg.DefaultServer)
	viper.Set("servers", cfg.Servers)

	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func ConfigPath() string {
	return configPath
}

// IsConfigured returns true if both server URL and token are set
func IsConfigured() bool {
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
