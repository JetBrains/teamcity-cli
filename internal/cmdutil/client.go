package cmdutil

import (
	"context"
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/version"
)

func (f *Factory) defaultGetClient() (api.ClientInterface, error) {
	serverURL := config.GetServerURL()
	token, source, keyringErr := config.GetTokenWithSource()

	debugOpt := api.WithDebugFunc(f.Printer.Debug)
	roOpt := api.WithReadOnly(config.IsReadOnly())
	verOpt := api.WithVersion(version.String())

	opts := []api.ClientOption{debugOpt, roOpt, verOpt}
	if h := api.EnvHeaders(); len(h) > 0 {
		opts = append(opts, api.WithExtraHeaders(h))
	}

	if config.IsGuestAuth() {
		if serverURL == "" {
			return nil, api.Validation(
				"TEAMCITY_GUEST is set but no server URL configured",
				fmt.Sprintf("Set %s environment variable or run 'teamcity auth login --guest -s <url>'", config.EnvServerURL),
			)
		}
		f.Printer.Debug("Using guest authentication")
		opts = append(opts, api.WithAuthSource(api.AuthSourceGuest))
		return api.NewGuestClient(serverURL, opts...).WithContext(f.Context()), nil
	}

	if serverURL != "" && token != "" {
		f.WarnInsecureHTTP(serverURL, "authentication token")
		opts = append(opts, api.WithAuthSource(resolveAuthSource(source)))
		return api.NewClient(serverURL, token, opts...).WithContext(f.Context()), nil
	}

	if buildAuth, ok := config.GetBuildAuth(); ok {
		if serverURL == "" {
			serverURL = buildAuth.ServerURL
		}
		f.Printer.Debug("Using build-level authentication")
		f.WarnInsecureHTTP(serverURL, "credentials")
		opts = append(opts, api.WithAuthSource(api.AuthSourceBuild))
		return api.NewClientWithBasicAuth(serverURL, buildAuth.Username, buildAuth.Password, opts...).WithContext(f.Context()), nil
	}

	return nil, NotAuthenticatedError(f.Context(), serverURL, keyringErr)
}

// resolveAuthSource maps a token-source string plus config state onto an api.AuthSource.
func resolveAuthSource(tokenSource string) api.AuthSource {
	if config.IsGuestAuth() {
		return api.AuthSourceGuest
	}
	if tokenSource == "env" {
		return api.AuthSourceEnv
	}
	if sc, ok := config.Get().Servers[config.GetServerURL()]; ok && sc.TokenExpiry != "" {
		return api.AuthSourcePKCE
	}
	return api.AuthSourceManual
}

// ProbeGuestAccess checks whether the server at serverURL supports guest access; honors ctx for cancellation.
func ProbeGuestAccess(ctx context.Context, serverURL string) bool {
	if serverURL == "" {
		return false
	}
	guest := api.NewGuestClient(serverURL, api.WithVersion(version.String())).WithContext(ctx)
	_, err := guest.GetServer()
	return err == nil
}

// NotAuthenticatedError returns a not-authenticated error with a hint that covers all authentication methods.
func NotAuthenticatedError(ctx context.Context, serverURL string, keyringErr error) *api.ValidationError {
	msg := "Not authenticated"
	if keyringErr != nil {
		msg = fmt.Sprintf("Not authenticated (could not access system keyring: %v)", keyringErr)
	}

	suggestion := "If you use environment overrides, set both TEAMCITY_URL and TEAMCITY_TOKEN; TEAMCITY_URL alone bypasses stored credentials. Otherwise unset TEAMCITY_URL to use stored auth, or run 'teamcity auth login --insecure-storage'"
	if ProbeGuestAccess(ctx, serverURL) {
		suggestion += ", or set TEAMCITY_GUEST=1 for guest access"
	}

	return api.Validation(msg, suggestion)
}
