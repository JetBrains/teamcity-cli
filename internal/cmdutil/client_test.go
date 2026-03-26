package cmdutil

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSOptionNoPaths(t *testing.T) {
	t.Setenv("TEAMCITY_URL", "https://tc.example.com")
	t.Setenv("TEAMCITY_CLIENT_CERT", "")
	t.Setenv("TEAMCITY_CLIENT_KEY", "")
	t.Setenv("TEAMCITY_CA_CERT", "")
	config.Init()

	opt, err := tlsOption()
	require.NoError(t, err)
	assert.Nil(t, opt)
}

func TestTLSOptionCertWithoutKey(t *testing.T) {
	t.Setenv("TEAMCITY_URL", "https://tc.example.com")
	t.Setenv("TEAMCITY_CLIENT_CERT", "/some/cert.pem")
	t.Setenv("TEAMCITY_CLIENT_KEY", "")
	t.Setenv("TEAMCITY_CA_CERT", "")
	config.Init()

	_, err := tlsOption()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "both client certificate and key must be provided together")
}

func TestTLSOptionKeyWithoutCert(t *testing.T) {
	t.Setenv("TEAMCITY_URL", "https://tc.example.com")
	t.Setenv("TEAMCITY_CLIENT_CERT", "")
	t.Setenv("TEAMCITY_CLIENT_KEY", "/some/key.pem")
	t.Setenv("TEAMCITY_CA_CERT", "")
	config.Init()

	_, err := tlsOption()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "both client certificate and key must be provided together")
}
