package config

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	EnvServerURL = "TEAMCITY_URL"
	EnvToken     = "TEAMCITY_TOKEN"
	EnvGuestAuth = "TEAMCITY_GUEST"
	EnvReadOnly  = "TEAMCITY_RO"
	EnvDSLDir    = "TEAMCITY_DSL_DIR"
	EnvProject   = "TEAMCITY_PROJECT"
	EnvJob       = "TEAMCITY_JOB"

	DefaultDSLDirTeamCity = ".teamcity"
	DefaultDSLDirTC       = ".tc"

	dslPluginsRepoSuffix = "/app/dsl-plugins-repository"
)

type ServerConfig struct {
	Token       string `mapstructure:"token"`
	User        string `mapstructure:"user"`
	Guest       bool   `mapstructure:"guest,omitempty"`
	RO          bool   `mapstructure:"ro,omitempty"`
	TokenExpiry string `mapstructure:"token_expiry,omitempty"`
}

type Config struct {
	DefaultServer        string                  `mapstructure:"default_server"`
	Servers              map[string]ServerConfig `mapstructure:"servers"`
	Aliases              map[string]string       `mapstructure:"aliases"`
	Analytics            *bool                   `mapstructure:"analytics,omitempty"`
	AnalyticsNoticeShown bool                    `mapstructure:"analytics_notice_shown,omitempty"`
}

var (
	cfg        *Config
	configPath string

	// vi uses "::" as key delimiter to avoid Viper splitting URL map keys on dots
	vi = viper.NewWithOptions(viper.KeyDelimiter("::"))

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

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	configDir := filepath.Join(configHome, "tc")
	configPath = filepath.Join(configDir, "config.yml")

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	vi.SetConfigFile(configPath)
	vi.SetConfigType("yaml")
	vi.SetDefault("servers", map[string]ServerConfig{})

	if err := vi.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to read config: %w", err)
			}
		}
	}

	cfg = &Config{}
	if err := vi.Unmarshal(cfg); err != nil {
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

// SortedServerURLs returns configured server URLs with the default server first, then the rest alphabetically.
func SortedServerURLs(c *Config) []string {
	urls := slices.Collect(maps.Keys(c.Servers))
	slices.SortFunc(urls, func(a, b string) int {
		if ad, bd := a == c.DefaultServer, b == c.DefaultServer; ad != bd {
			if ad {
				return -1
			}
			return 1
		}
		return cmp.Compare(a, b)
	})
	return urls
}

// NormalizeURL reduces a user-supplied URL to its origin (scheme + host + port), adding https:// if no scheme.
//
//goland:noinspection HttpUrlsUsage
func NormalizeURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		return strings.TrimSuffix(u, "/")
	}
	return parsed.Scheme + "://" + parsed.Host
}

func keyringService(serverURL string) string {
	return "tc:" + serverURL
}

// GetServerURL resolves the target server from TEAMCITY_URL, then the configured default; never from DSL (avoids routing a stored token to an untrusted repo's .teamcity/pom.xml — opt in via `auth login`).
func GetServerURL() string {
	if serverUrl := os.Getenv(EnvServerURL); serverUrl != "" {
		return NormalizeURL(serverUrl)
	}
	return cfg.DefaultServer
}

func GetToken() string {
	token, _, _ := GetTokenWithSource()
	return token
}

func GetTokenWithSource() (token, source string, keyringErr error) {
	if token := os.Getenv(EnvToken); token != "" {
		return token, "env", nil
	}

	serverURL := GetServerURL()
	if serverURL == "" {
		return "", "", nil
	}

	server, ok := cfg.Servers[serverURL]
	if ok && server.User != "" {
		t, err := keyringGet(keyringService(serverURL), server.User)
		if err == nil && t != "" {
			return t, "keyring", nil
		}
		if err != nil && !errors.Is(err, errKeyringNotFound) {
			keyringErr = err
		}
	}

	if ok && server.Token != "" {
		return server.Token, "config", nil
	}
	return "", "", keyringErr
}

// GetTokenForServer retrieves the token for a specific server URL.
// Unlike GetTokenWithSource, it does not use GetServerURL() — the caller
// provides the server URL directly. Returns the token and its source
// ("keyring" or "config"), or empty strings if none found.
func GetTokenForServer(serverURL string) (token, source string, keyringErr error) {
	server, ok := cfg.Servers[serverURL]
	if ok && server.User != "" {
		t, err := keyringGet(keyringService(serverURL), server.User)
		if err == nil && t != "" {
			return t, "keyring", nil
		}
		if err != nil && !errors.Is(err, errKeyringNotFound) {
			keyringErr = err
		}
	}
	if ok && server.Token != "" {
		return server.Token, "config", nil
	}
	return "", "", keyringErr
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
	_, err := SetServerWithKeyring(serverURL, token, user, "", false)
	return err
}

func SetServerWithKeyring(serverURL, token, user, tokenExpiry string, insecureStorage bool) (insecureFallback bool, err error) {
	serverURL = NormalizeURL(serverURL)
	cfg.DefaultServer = serverURL

	if !insecureStorage {
		if krErr := keyringSet(keyringService(serverURL), user, token); krErr == nil {
			cfg.Servers[serverURL] = ServerConfig{User: user, TokenExpiry: tokenExpiry}
			return false, writeConfig()
		}
	}

	cfg.Servers[serverURL] = ServerConfig{Token: token, User: user, TokenExpiry: tokenExpiry}
	return true, writeConfig()
}

func GetTokenExpiry() string {
	if server, ok := cfg.Servers[GetServerURL()]; ok {
		return server.TokenExpiry
	}
	return ""
}

func serverConfigToMap(sc ServerConfig) map[string]any {
	m := map[string]any{}
	if sc.Token != "" {
		m["token"] = sc.Token
	}
	if sc.User != "" {
		m["user"] = sc.User
	}
	if sc.Guest {
		m["guest"] = true
	}
	if sc.RO {
		m["ro"] = true
	}
	if sc.TokenExpiry != "" {
		m["token_expiry"] = sc.TokenExpiry
	}
	return m
}

func writeConfig() error {
	// Use a fresh viper instance for writing to avoid stale keys from the
	// initial file read (viper merges rather than replaces nested maps).
	w := viper.NewWithOptions(viper.KeyDelimiter("::"))
	w.SetConfigType("yaml")

	w.Set("default_server", cfg.DefaultServer)

	servers := make(map[string]any, len(cfg.Servers))
	for serverUrl, sc := range cfg.Servers {
		servers[serverUrl] = serverConfigToMap(sc)
	}
	w.Set("servers", servers)
	w.Set("aliases", cfg.Aliases)
	if cfg.Analytics != nil {
		w.Set("analytics", *cfg.Analytics)
	}
	if cfg.AnalyticsNoticeShown {
		w.Set("analytics_notice_shown", true)
	}

	data, err := yaml.Marshal(w.AllSettings())
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return writeFileAtomic0600(configPath, data)
}

// writeFileAtomic0600 writes data via a 0600 sibling temp file + rename, so a token-bearing config is never exposed at 0644 and concurrent writers can't interleave. If path is a symlink, writes go to the resolved target so dotfile-managed setups don't lose their link on first write.
func writeFileAtomic0600(path string, data []byte) error {
	path = resolveSymlink(path)
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// resolveSymlink returns the symlink target if path is a symlink, else path
// unchanged. Handles dangling links via Readlink so a pre-configured but
// never-written symlink is still preserved on first write.
func resolveSymlink(path string) string {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return path
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	target, err := os.Readlink(path)
	if err != nil {
		return path
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return target
}

func RemoveServer(serverURL string) error {
	if server, ok := cfg.Servers[serverURL]; ok && server.User != "" {
		_ = keyringDelete(keyringService(serverURL), server.User)
	}

	delete(cfg.Servers, serverURL)

	if cfg.DefaultServer == serverURL {
		cfg.DefaultServer = ""
		if urls := slices.Sorted(maps.Keys(cfg.Servers)); len(urls) > 0 {
			cfg.DefaultServer = urls[0]
		}
	}

	return writeConfig()
}

func ConfigPath() string {
	return configPath
}

// IsAnalyticsEnabled returns the user's persisted preference; defaults to true.
func IsAnalyticsEnabled() bool {
	if cfg == nil || cfg.Analytics == nil {
		return true
	}
	return *cfg.Analytics
}

func SetAnalyticsEnabled(enabled bool) error {
	cfg.Analytics = &enabled
	return writeConfig()
}

func IsAnalyticsNoticeShown() bool {
	if cfg == nil {
		return false
	}
	return cfg.AnalyticsNoticeShown
}

func MarkAnalyticsNoticeShown() error {
	if cfg.AnalyticsNoticeShown {
		return nil
	}
	cfg.AnalyticsNoticeShown = true
	return writeConfig()
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

// IsReadOnly returns true if read-only mode is enabled via env var or server config.
// When enabled, all non-GET API requests are blocked.
func IsReadOnly() bool {
	if v := os.Getenv(EnvReadOnly); v == "1" || v == "true" || v == "yes" {
		return true
	}
	serverURL := GetServerURL()
	if serverURL == "" || cfg == nil {
		return false
	}
	if server, ok := cfg.Servers[serverURL]; ok {
		return server.RO
	}
	return false
}

// SetGuestServer saves a server with guest auth enabled and no token
func SetGuestServer(serverURL string) error {
	serverURL = NormalizeURL(serverURL)
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
	vi = viper.NewWithOptions(viper.KeyDelimiter("::"))
}
