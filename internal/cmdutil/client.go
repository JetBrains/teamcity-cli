package cmdutil

import (
	"fmt"
	"net/http"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

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
		WarnInsecureHTTP(serverURL, "authentication token")
		return api.NewClient(serverURL, token, opts...), nil
	}

	if buildAuth, ok := config.GetBuildAuth(); ok {
		if serverURL == "" {
			serverURL = buildAuth.ServerURL
		}
		output.Debug("Using build-level authentication")
		WarnInsecureHTTP(serverURL, "credentials")
		return api.NewClientWithBasicAuth(serverURL, buildAuth.Username, buildAuth.Password, opts...), nil
	}

	return nil, NotAuthenticatedError(serverURL, keyringErr)
}

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

// ProbeGuestAccess checks whether the server at serverURL supports guest access.
func ProbeGuestAccess(serverURL string) bool {
	if serverURL == "" {
		return false
	}
	guest := api.NewGuestClient(serverURL, api.WithDebugFunc(output.Debug))
	_, err := guest.GetServer()
	return err == nil
}

// NotAuthenticatedError returns a not-authenticated error with a hint that covers
// all authentication methods: environment variables, interactive login, and guest access.
func NotAuthenticatedError(serverURL string, keyringErr error) *tcerrors.UserError {
	msg := "Not authenticated"
	if keyringErr != nil {
		msg = fmt.Sprintf("Not authenticated (could not access system keyring: %v)", keyringErr)
	}

	suggestion := "Set TEAMCITY_URL and TEAMCITY_TOKEN environment variables, or run 'teamcity auth login --insecure-storage'"
	if ProbeGuestAccess(serverURL) {
		suggestion += ", or set TEAMCITY_GUEST=1 for guest access"
	}

	return &tcerrors.UserError{
		Message:    msg,
		Suggestion: suggestion,
	}
}
