package config

import (
	"net/url"
	"os"

	"github.com/magiconair/properties"
)

const (
	EnvBuildURL            = "BUILD_URL"
	EnvBuildPropertiesFile = "TEAMCITY_BUILD_PROPERTIES_FILE"
)

type BuildAuth struct {
	ServerURL string
	Username  string
	Password  string
}

func IsBuildEnvironment() bool {
	return os.Getenv(EnvBuildPropertiesFile) != ""
}

// GetBuildAuth returns build-level credentials if running inside a TeamCity build.
// Server URL priority: TEAMCITY_URL > BUILD_URL > teamcity.serverUrl property
func GetBuildAuth() (*BuildAuth, bool) {
	propsFile := os.Getenv(EnvBuildPropertiesFile)
	if propsFile == "" {
		return nil, false
	}

	props, err := properties.LoadFile(propsFile, properties.UTF8)
	if err != nil {
		return nil, false
	}

	username := props.GetString("teamcity.auth.userId", "")
	password := props.GetString("teamcity.auth.password", "")
	if username == "" || password == "" {
		return nil, false
	}

	serverURL := os.Getenv(EnvServerURL)
	if serverURL == "" {
		serverURL = extractServerURL(os.Getenv(EnvBuildURL))
	}
	if serverURL == "" {
		serverURL = props.GetString("teamcity.serverUrl", "")
	}
	if serverURL == "" {
		return nil, false
	}

	return &BuildAuth{
		ServerURL: serverURL,
		Username:  username,
		Password:  password,
	}, true
}

// extractServerURL extracts base URL from BUILD_URL.
func extractServerURL(buildURL string) string {
	if buildURL == "" {
		return ""
	}
	parsed, err := url.Parse(buildURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
