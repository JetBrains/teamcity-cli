package cmd

import (
	"fmt"
	"net/http"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

// GetClientFunc is the function used to create API clients.
// It can be overridden in tests to inject mock clients.
var GetClientFunc = defaultGetClient

// getClient returns an API client using the current GetClientFunc.
func getClient() (api.ClientInterface, error) {
	return GetClientFunc()
}

func defaultGetClient() (api.ClientInterface, error) {
	serverURL := config.GetServerURL()
	token, _, keyringErr := config.GetTokenWithSource()

	debugOpt := api.WithDebugFunc(output.Debug)
	roOpt := api.WithReadOnly(config.IsReadOnly())

	opts := []api.ClientOption{debugOpt, roOpt}

	tlsOpt, err := tlsOption()
	if err != nil {
		return nil, err
	}
	if tlsOpt != nil {
		opts = append(opts, tlsOpt)
	}

	if config.IsGuestAuth() {
		if serverURL == "" {
			return nil, tcerrors.WithSuggestion(
				"TEAMCITY_GUEST is set but no server URL configured",
				fmt.Sprintf("Set %s environment variable or run 'teamcity auth login --guest -s <url>'", config.EnvServerURL),
			)
		}
		output.Debug("Using guest authentication")
		return api.NewGuestClient(serverURL, opts...), nil
	}

	if serverURL != "" && token != "" {
		warnInsecureHTTP(serverURL, "authentication token")
		return api.NewClient(serverURL, token, opts...), nil
	}

	if buildAuth, ok := config.GetBuildAuth(); ok {
		if serverURL == "" {
			serverURL = buildAuth.ServerURL
		}
		output.Debug("Using build-level authentication")
		warnInsecureHTTP(serverURL, "credentials")
		return api.NewClientWithBasicAuth(serverURL, buildAuth.Username, buildAuth.Password, opts...), nil
	}

	return nil, notAuthenticatedError(serverURL, keyringErr)
}

// tlsOption returns a ClientOption that configures mTLS if TLS paths are
// configured via environment variables or per-server config.
func tlsOption() (api.ClientOption, error) {
	certFile, keyFile, caFile := config.GetTLSPaths()

	if certFile == "" && keyFile == "" && caFile == "" {
		return nil, nil
	}

	if (certFile == "") != (keyFile == "") {
		return nil, fmt.Errorf("both client certificate and key must be provided together")
	}

	tlsCfg, err := api.TLSConfig(certFile, keyFile, caFile)
	if err != nil {
		return nil, err
	}

	output.Debug("Using mTLS client certificate authentication")
	return api.WithTransport(&http.Transport{TLSClientConfig: tlsCfg}), nil
}
