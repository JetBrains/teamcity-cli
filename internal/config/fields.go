package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

var validKeys = []string{"default_server", "guest", "ro", "token_expiry"}

func IsValidKey(key string) bool {
	return slices.Contains(validKeys, key)
}

func ValidKeys() []string {
	return validKeys
}

func GetField(key, serverURL string) (string, error) {
	if !IsValidKey(key) {
		return "", unknownKeyError(key)
	}
	if key == "default_server" {
		return Get().DefaultServer, nil
	}
	serverURL, err := resolveServerForConfig(serverURL)
	if err != nil {
		return "", err
	}
	sc, ok := Get().Servers[serverURL]
	if !ok {
		return "", fmt.Errorf("server %q not found in configuration", serverURL)
	}
	switch key {
	case "guest":
		return fmt.Sprintf("%t", sc.Guest), nil
	case "ro":
		return fmt.Sprintf("%t", sc.RO), nil
	case "token_expiry":
		return sc.TokenExpiry, nil
	}
	return "", nil
}

func SetField(key, value, serverURL string) error {
	if !IsValidKey(key) {
		return unknownKeyError(key)
	}
	if key == "default_server" {
		if value == "" {
			return fmt.Errorf("value cannot be empty")
		}
		normalized := NormalizeURL(value)
		if _, ok := cfg.Servers[normalized]; !ok {
			return fmt.Errorf("server %q not found in configuration; run 'teamcity auth login --server %s' first", normalized, value)
		}
		cfg.DefaultServer = normalized
		return writeConfig()
	}
	serverURL, err := resolveServerForConfig(serverURL)
	if err != nil {
		return err
	}
	sc, ok := cfg.Servers[serverURL]
	if !ok {
		return fmt.Errorf("server %q not found in configuration", serverURL)
	}
	switch key {
	case "guest":
		b, err := parseBoolValue(value)
		if err != nil {
			return err
		}
		sc.Guest = b
	case "ro":
		b, err := parseBoolValue(value)
		if err != nil {
			return err
		}
		sc.RO = b
	case "token_expiry":
		sc.TokenExpiry = value
	}
	cfg.Servers[serverURL] = sc
	return writeConfig()
}

func ResetField(key, serverURL string) error {
	if !IsValidKey(key) {
		return unknownKeyError(key)
	}
	if key == "default_server" {
		cfg.DefaultServer = ""
		if urls := slices.Sorted(maps.Keys(cfg.Servers)); len(urls) > 0 {
			cfg.DefaultServer = urls[0]
		}
		return writeConfig()
	}
	serverURL, err := resolveServerForConfig(serverURL)
	if err != nil {
		return err
	}
	sc, ok := cfg.Servers[serverURL]
	if !ok {
		return fmt.Errorf("server %q not found in configuration", serverURL)
	}
	switch key {
	case "guest":
		sc.Guest = false
	case "ro":
		sc.RO = false
	case "token_expiry":
		sc.TokenExpiry = ""
	}
	cfg.Servers[serverURL] = sc
	return writeConfig()
}

func resolveServerForConfig(serverURL string) (string, error) {
	if serverURL != "" {
		return NormalizeURL(serverURL), nil
	}
	c := Get()
	if c.DefaultServer == "" {
		return "", fmt.Errorf("no default server configured; use --server flag or run 'teamcity auth login'")
	}
	return c.DefaultServer, nil
}

func parseBoolValue(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q; use true or false", s)
	}
}

func unknownKeyError(key string) error {
	return fmt.Errorf("unknown key %q; valid keys: %s", key, strings.Join(validKeys, ", "))
}
