package onboard

import (
	"errors"
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd/auth"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/version"
)

// accessToken exchanges the onboarding token, or uses the saved login when none is given or the exchange is rejected.
func accessToken(f *cmdutil.Factory, serverURL, onboardingToken string) (string, error) {
	if onboardingToken == "" {
		return savedLogin(f, serverURL, "")
	}

	token, exchangeErr := auth.ExchangeToken(f, serverURL, onboardingToken)
	if !errors.Is(exchangeErr, auth.ErrExchangeRejected) {
		return token, exchangeErr
	}
	saved, err := savedLogin(f, serverURL, fmt.Sprintf("couldn't exchange the onboarding token (%s) and ", exchangeErr))
	if err != nil {
		return "", err
	}
	f.Printer.Warn("Couldn't exchange the onboarding token; using your existing login for %s", serverURL)
	return saved, nil
}

// savedLogin returns the stored token for serverURL if it still authenticates; reason prefixes the not-logged-in error.
func savedLogin(f *cmdutil.Factory, serverURL, reason string) (string, error) {
	saved, _, err := config.GetTokenForServer(serverURL)
	if err != nil {
		return "", fmt.Errorf("read token for %s: %w", serverURL, err)
	}
	if saved == "" || !loginWorks(f, serverURL, saved) {
		return "", api.Validation(
			fmt.Sprintf("%syou're not logged in to %s", reason, serverURL),
			fmt.Sprintf("Run teamcity auth login -s %s, then teamcity onboard %s", serverURL, serverURL),
		)
	}
	return saved, nil
}

// loginWorks reports whether token authenticates against serverURL.
func loginWorks(f *cmdutil.Factory, serverURL, token string) bool {
	client := api.NewClient(serverURL, token, api.WithDebugFunc(f.Printer.Debug), api.WithVersion(version.String())).WithContext(f.Context())
	_, err := client.GetCurrentUser()
	return err == nil
}
