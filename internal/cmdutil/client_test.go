package cmdutil

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestResolveAuthSource(t *testing.T) {
	const serverURL = "https://tc.example.com"

	tests := []struct {
		name        string
		tokenSource string
		guestEnv    string
		tokenExpiry string
		want        api.AuthSource
	}{
		{"guest via env", "config", "1", "", api.AuthSourceGuest},
		{"env token", "env", "", "", api.AuthSourceEnv},
		{"pkce token with expiry", "keyring", "", "2030-01-01T00:00:00Z", api.AuthSourcePKCE},
		{"manual token without expiry", "keyring", "", "", api.AuthSourceManual},
		{"manual when source is config", "config", "", "", api.AuthSourceManual},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(config.EnvServerURL, serverURL)
			t.Setenv(config.EnvGuestAuth, tc.guestEnv)

			config.ResetForTest()
			cfg := config.Get()
			cfg.DefaultServer = serverURL
			cfg.Servers[serverURL] = config.ServerConfig{
				User:        "alice",
				TokenExpiry: tc.tokenExpiry,
			}
			t.Cleanup(config.ResetForTest)

			assert.Equal(t, tc.want, resolveAuthSource(tc.tokenSource))
		})
	}
}
