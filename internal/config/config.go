package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	EnvServerURL = "TEAMCITY_URL"
	EnvToken     = "TEAMCITY_TOKEN"
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
