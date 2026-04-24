package cmdutil

import (
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

func TestParseExtraHeaders(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name    string
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "single header with space after colon",
			input: []string{"CF-Access-Client-Id: team.access"},
			want:  map[string]string{"CF-Access-Client-Id": "team.access"},
		},
		{
			name:  "single header without space after colon",
			input: []string{"CF-Access-Client-Id:team.access"},
			want:  map[string]string{"CF-Access-Client-Id": "team.access"},
		},
		{
			name: "multiple headers",
			input: []string{
				"CF-Access-Client-Id: team.access",
				"CF-Access-Client-Secret: secret123",
			},
			want: map[string]string{
				"CF-Access-Client-Id":     "team.access",
				"CF-Access-Client-Secret": "secret123",
			},
		},
		{
			name:  "value containing a colon",
			input: []string{"Authorization: Bearer tok:en"},
			want:  map[string]string{"Authorization": "Bearer tok:en"},
		},
		{
			name:    "missing colon separator",
			input:   []string{"CF-Access-Client-Id"},
			wantErr: true,
		},
		{
			name:    "empty name",
			input:   []string{": value"},
			wantErr: true,
		},
		{
			name:  "empty input",
			input: []string{},
			want:  map[string]string{},
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseExtraHeaders(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
