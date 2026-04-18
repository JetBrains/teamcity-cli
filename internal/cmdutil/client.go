package cmdutil

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/version"
)

func (f *Factory) defaultGetClient() (api.ClientInterface, error) {
	serverURL := config.GetServerURL()
	token, _, keyringErr := config.GetTokenWithSource()

	debugOpt := api.WithDebugFunc(f.Printer.Debug)
	roOpt := api.WithReadOnly(config.IsReadOnly())
	verOpt := api.WithVersion(version.String())

	opts := []api.ClientOption{debugOpt, roOpt, verOpt}

	if config.IsGuestAuth() {
		if serverURL == "" {
			return nil, tcerrors.WithSuggestion(
				"TEAMCITY_GUEST is set but no server URL configured",
				fmt.Sprintf("Set %s environment variable or run 'teamcity auth login --guest -s <url>'", config.EnvServerURL),
			)
		}
		f.Printer.Debug("Using guest authentication")
		return api.NewGuestClient(serverURL, opts...), nil
	}

	if serverURL != "" && token != "" {
		f.WarnInsecureHTTP(serverURL, "authentication token")
		return api.NewClient(serverURL, token, opts...), nil
	}

	if buildAuth, ok := config.GetBuildAuth(); ok {
		if serverURL == "" {
			serverURL = buildAuth.ServerURL
		}
		f.Printer.Debug("Using build-level authentication")
		f.WarnInsecureHTTP(serverURL, "credentials")
		return api.NewClientWithBasicAuth(serverURL, buildAuth.Username, buildAuth.Password, opts...), nil
	}

	return nil, NotAuthenticatedError(serverURL, keyringErr)
}

// ProbeGuestAccess checks whether the server at serverURL supports guest access.
func ProbeGuestAccess(serverURL string) bool {
	if serverURL == "" {
		return false
	}
	guest := api.NewGuestClient(serverURL, api.WithVersion(version.String()))
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

	suggestion := "If you use environment overrides, set both TEAMCITY_URL and TEAMCITY_TOKEN. Otherwise unset TEAMCITY_URL to use stored auth, or run 'teamcity auth login --insecure-storage'"
	if ProbeGuestAccess(serverURL) {
		suggestion += ", or set TEAMCITY_GUEST=1 for guest access"
	}

	return &tcerrors.UserError{
		Message:    msg,
		Suggestion: suggestion,
	}
}
