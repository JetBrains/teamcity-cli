package cmdutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDefaultGetClient_AppliesEnvHeaders(t *testing.T) {
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	t.Setenv(config.EnvServerURL, server.URL)
	t.Setenv(config.EnvToken, "tok")
	t.Setenv("TEAMCITY_HEADER_CF_ACCESS_CLIENT_ID", "abc.id")
	t.Setenv("TEAMCITY_HEADER_CF_ACCESS_CLIENT_SECRET", "shh")

	config.ResetForTest()
	t.Cleanup(config.ResetForTest)

	f := NewFactory()
	client, err := f.Client()
	require.NoError(t, err)

	_, err = client.GetServer()
	require.NoError(t, err)

	assert.Equal(t, "abc.id", got.Get("Cf-Access-Client-Id"))
	assert.Equal(t, "shh", got.Get("Cf-Access-Client-Secret"))
}
