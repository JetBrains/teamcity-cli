package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBuildEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "returns true when env var is set",
			envValue: "/path/to/build.properties",
			want:     true,
		},
		{
			name:     "returns false when env var is empty",
			envValue: "",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvBuildPropertiesFile, tc.envValue)
			got := IsBuildEnvironment()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractServerURL(t *testing.T) {
	tests := []struct {
		name     string
		buildURL string
		want     string
	}{
		{
			name:     "extracts base URL from viewLog URL",
			buildURL: "https://teamcity.example.com/viewLog.html?buildId=12345&buildTypeId=Project_Build",
			want:     "https://teamcity.example.com",
		},
		{
			name:     "handles URL with port",
			buildURL: "https://teamcity.example.com:8443/viewLog.html?buildId=12345",
			want:     "https://teamcity.example.com:8443",
		},
		{
			name:     "handles HTTP URL",
			buildURL: "http://localhost:8111/viewLog.html?buildId=99",
			want:     "http://localhost:8111",
		},
		{
			name:     "returns empty for empty URL",
			buildURL: "",
			want:     "",
		},
		{
			name:     "returns empty for invalid URL",
			buildURL: "not-a-valid-url",
			want:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractServerURL(tc.buildURL)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetBuildAuth(t *testing.T) {
	t.Run("returns auth from properties file", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `teamcity.auth.userId=buildUser123
teamcity.auth.password=buildPass456
teamcity.serverUrl=https://tc.example.com
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		t.Setenv(EnvBuildPropertiesFile, propsFile)
		t.Setenv(EnvServerURL, "")
		t.Setenv(EnvBuildURL, "")

		auth, ok := GetBuildAuth()
		require.True(t, ok)
		assert.Equal(t, "https://tc.example.com", auth.ServerURL)
		assert.Equal(t, "buildUser123", auth.Username)
		assert.Equal(t, "buildPass456", auth.Password)
	})

	t.Run("prefers TEAMCITY_URL over BUILD_URL and properties", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `teamcity.auth.userId=user
teamcity.auth.password=pass
teamcity.serverUrl=https://props.example.com
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		t.Setenv(EnvBuildPropertiesFile, propsFile)
		t.Setenv(EnvServerURL, "https://explicit.example.com")
		t.Setenv(EnvBuildURL, "https://buildurl.example.com/viewLog.html?buildId=123")

		auth, ok := GetBuildAuth()
		require.True(t, ok)
		assert.Equal(t, "https://explicit.example.com", auth.ServerURL)
	})

	t.Run("prefers BUILD_URL over properties for server URL", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `teamcity.auth.userId=user
teamcity.auth.password=pass
teamcity.serverUrl=https://props.example.com
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		t.Setenv(EnvBuildPropertiesFile, propsFile)
		t.Setenv(EnvServerURL, "")
		t.Setenv(EnvBuildURL, "https://buildurl.example.com/viewLog.html?buildId=123")

		auth, ok := GetBuildAuth()
		require.True(t, ok)
		assert.Equal(t, "https://buildurl.example.com", auth.ServerURL)
	})

	t.Run("returns false when env var not set", func(t *testing.T) {
		t.Setenv(EnvBuildPropertiesFile, "")

		_, ok := GetBuildAuth()
		assert.False(t, ok)
	})

	t.Run("returns false when file does not exist", func(t *testing.T) {
		t.Setenv(EnvBuildPropertiesFile, "/nonexistent/path/build.properties")

		_, ok := GetBuildAuth()
		assert.False(t, ok)
	})

	t.Run("returns false when credentials missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `teamcity.auth.userId=user
teamcity.serverUrl=https://tc.example.com
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		t.Setenv(EnvBuildPropertiesFile, propsFile)
		t.Setenv(EnvBuildURL, "")

		_, ok := GetBuildAuth()
		assert.False(t, ok)
	})

	t.Run("returns false when server URL missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `teamcity.auth.userId=user
teamcity.auth.password=pass
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		t.Setenv(EnvBuildPropertiesFile, propsFile)
		t.Setenv(EnvServerURL, "")
		t.Setenv(EnvBuildURL, "")

		_, ok := GetBuildAuth()
		assert.False(t, ok)
	})

	t.Run("handles properties with special characters", func(t *testing.T) {
		tmpDir := t.TempDir()
		propsFile := filepath.Join(tmpDir, "build.properties")

		content := `teamcity.auth.userId=user@domain.com
teamcity.auth.password=p\=ss:word
teamcity.serverUrl=https://tc.example.com
`
		err := os.WriteFile(propsFile, []byte(content), 0600)
		require.NoError(t, err)

		t.Setenv(EnvBuildPropertiesFile, propsFile)
		t.Setenv(EnvBuildURL, "")

		auth, ok := GetBuildAuth()
		require.True(t, ok)
		assert.Equal(t, "user@domain.com", auth.Username)
		assert.Contains(t, auth.Password, "ss")
	})
}
