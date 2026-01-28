package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	EnvServerURL             = "TEAMCITY_URL"
	EnvToken                 = "TEAMCITY_TOKEN"
	EnvBuildPropertiesFile   = "TEAMCITY_BUILD_PROPERTIES_FILE"
	BuildAuthUserIDProperty  = "teamcity.auth.userId"
	BuildAuthPasswordProperty = "teamcity.auth.password"
	TeamCityServerURLProperty = "teamcity.serverUrl"
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
)

func Init() error {
	home, err := os.UserHomeDir()
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

	if cfg.Servers == nil {
		cfg.Servers = make(map[string]ServerConfig)
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

func GetServerURL() string {
	if url := os.Getenv(EnvServerURL); url != "" {
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

// BuildAuth contains TeamCity build-level authentication credentials.
// These credentials are provided by TeamCity to running builds and allow
// R/O access to the project's artifacts.
type BuildAuth struct {
	ServerURL string
	UserID    string
	Password  string
}

// GetBuildAuth reads TeamCity build-level authentication credentials from
// the build properties file specified by TEAMCITY_BUILD_PROPERTIES_FILE.
// Returns nil if not running in a TeamCity build environment or if
// credentials are not available.
func GetBuildAuth() (*BuildAuth, error) {
	propsFile := os.Getenv(EnvBuildPropertiesFile)
	if propsFile == "" {
		return nil, fmt.Errorf("not running in a TeamCity build environment (TEAMCITY_BUILD_PROPERTIES_FILE not set)")
	}

	props, err := readPropertiesFile(propsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read build properties file: %w", err)
	}

	userID := props[BuildAuthUserIDProperty]
	password := props[BuildAuthPasswordProperty]

	if userID == "" || password == "" {
		return nil, fmt.Errorf("build auth credentials not found in properties file")
	}

	// Get server URL from properties or fall back to environment/config
	serverURL := props[TeamCityServerURLProperty]
	if serverURL == "" {
		serverURL = GetServerURL()
	}
	if serverURL == "" {
		return nil, fmt.Errorf("TeamCity server URL not found (set TEAMCITY_URL or use teamcity.serverUrl in build properties)")
	}

	return &BuildAuth{
		ServerURL: strings.TrimSuffix(serverURL, "/"),
		UserID:    userID,
		Password:  password,
	}, nil
}

// readPropertiesFile reads a Java-style properties file and returns a map of key-value pairs.
// It handles basic property file format: key=value, with # or ! for comments.
func readPropertiesFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	props := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		// Handle continuation lines (ending with \)
		for strings.HasSuffix(line, "\\") && scanner.Scan() {
			line = strings.TrimSuffix(line, "\\") + strings.TrimSpace(scanner.Text())
		}

		// Find separator (= or :)
		sepIdx := strings.IndexAny(line, "=:")
		if sepIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:sepIdx])
		value := strings.TrimSpace(line[sepIdx+1:])

		// Unescape common escape sequences
		value = unescapePropertyValue(value)

		props[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return props, nil
}

// unescapePropertyValue handles common Java properties escape sequences.
func unescapePropertyValue(s string) string {
	replacer := strings.NewReplacer(
		"\\n", "\n",
		"\\r", "\r",
		"\\t", "\t",
		"\\\\", "\\",
		"\\=", "=",
		"\\:", ":",
	)
	return replacer.Replace(s)
}
