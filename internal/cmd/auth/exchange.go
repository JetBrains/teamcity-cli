package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/version"
)

// regenerateTip is shown on every exchange failure: the onboarding token is consumed even when the exchange fails.
const regenerateTip = "Generate a new onboarding token in TeamCity and rerun teamcity onboard"

// ExchangeToken trades a short-lived onboarding token for a TeamCity access token and stores it like `auth login`.
func ExchangeToken(f *cmdutil.Factory, serverURL, token string) (string, error) {
	p := f.Printer
	ctx := f.Context()
	f.WarnInsecureHTTP(serverURL, "onboarding token")

	newClient := func(tok string) *api.Client {
		return api.NewClient(serverURL, tok,
			api.WithDebugFunc(p.Debug),
			api.WithVersion(version.String()),
		).WithContext(ctx)
	}

	p.Progress("Exchanging onboarding token... ")
	exchanged, err := exchangeOneTimeToken(newClient(token))
	if err != nil {
		p.Info("%s", output.Red(output.Sym().Cross))
		return "", exchangeError(err, serverURL)
	}
	p.Info("%s", output.Green(output.Sym().Check))

	client := newClient(exchanged.Value)
	user, err := client.GetCurrentUser()
	if err != nil {
		return "", fmt.Errorf("failed to verify access token: %w", err)
	}

	insecureFallback, err := config.SetServerWithKeyring(serverURL, exchanged.Value, user.Username, "", false)
	if err != nil {
		return "", fmt.Errorf("failed to save configuration: %w", err)
	}
	cacheServerInfo(serverURL, client)

	p.Success("Logged in to %s as %s", output.Cyan(serverURL), output.Cyan(user.Name))
	if insecureFallback {
		p.Warn("Token stored in plain text at %s", config.ConfigPath())
	} else {
		p.Success("Token stored in system keyring")
	}
	return exchanged.Value, nil
}

// exchangeOneTimeToken trades the one-time token of client for a token limited to onboardingTokenPermissions.
// Workaround for TW-105776 (CLI side: TW-105780); once the server restricts exchanged tokens, go back to the regular
// request: client.ExchangeOneTimeToken on POST /app/rest/users/current/tokens with the name only, without the user id and permissions.
func exchangeOneTimeToken(client *api.Client) (*api.Token, error) {
	userID, err := client.GetCurrentUserID()
	if err != nil {
		return nil, err
	}
	return client.ExchangeOneTimeToken(userID, "teamcity-cli-"+rand.Text(), onboardingTokenPermissions)
}

// ErrExchangeRejected marks a 401 from the exchange endpoint (token used or expired), so callers may fall back to a saved login.
var ErrExchangeRejected = errors.New("onboarding token exchange rejected")

// rejectedError tags a validation error with ErrExchangeRejected; it still unwraps to *api.ValidationError for rendering.
type rejectedError struct{ *api.ValidationError }

func (e rejectedError) Unwrap() []error { return []error{ErrExchangeRejected, e.ValidationError} }

// exchangeError maps exchange endpoint failures to actionable errors.
func exchangeError(err error, serverURL string) error {
	if he, ok := errors.AsType[*api.HTTPError](err); ok && he.Status == http.StatusBadRequest {
		return api.Validation(
			"onboarding token exchange failed: "+he.Error(),
			fmt.Sprintf("%s; if the token limit is reached, first delete unused tokens at %s/profile.html?item=accessTokens", regenerateTip, serverURL),
		)
	}
	if ue, ok := errors.AsType[api.UserError](err); ok && ue.Category() == api.CatAuth {
		return rejectedError{api.Validation("onboarding token is expired or invalid", regenerateTip)}
	}
	return err
}
