package analytics

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	EnvAnalytics  = "TEAMCITY_ANALYTICS"
	EnvDoNotTrack = "DO_NOT_TRACK"
	EnvStaging    = "TEAMCITY_ANALYTICS_STAGING"
	EnvSalt       = "TEAMCITY_ANALYTICS_SALT"
)

const (
	ConfigKey      = "analytics"
	NoticeShownKey = "analytics_notice_shown"
)

type OptOutReason string

const (
	OptOutNone       OptOutReason = ""
	OptOutDoNotTrack OptOutReason = "DO_NOT_TRACK"
	OptOutEnv        OptOutReason = "TEAMCITY_ANALYTICS=0"
	OptOutConfig     OptOutReason = "config: analytics=false"
)

// IsEnabled reports whether analytics tracking is on; precedence: DO_NOT_TRACK > TEAMCITY_ANALYTICS > config > on.
func IsEnabled(configEnabled bool) (bool, OptOutReason) {
	if isTruthy(os.Getenv(EnvDoNotTrack)) {
		return false, OptOutDoNotTrack
	}
	if v := os.Getenv(EnvAnalytics); v != "" && !isTruthy(v) {
		return false, OptOutEnv
	}
	if !configEnabled {
		return false, OptOutConfig
	}
	return true, OptOutNone
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// IsStaging reports whether to route events to the FUS staging endpoint.
func IsStaging() bool {
	return isTruthy(os.Getenv(EnvStaging))
}

// Salt returns the anonymization salt from $TEAMCITY_ANALYTICS_SALT, empty if unset.
func Salt() string {
	return os.Getenv(EnvSalt)
}

// DataDir returns ($XDG_CONFIG_HOME|~/.config)/tc/.analytics, creating it if missing.
func DataDir() (string, error) {
	base, err := configHomeBase()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "tc", ".analytics")
	return dir, os.MkdirAll(dir, 0o700)
}

// SessionFilePath returns the persisted application-session record path.
func SessionFilePath() (string, error) {
	base, err := configHomeBase()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "tc")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, ".session"), nil
}

func configHomeBase() (string, error) {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return base, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}
