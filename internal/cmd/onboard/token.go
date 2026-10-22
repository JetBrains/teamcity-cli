package onboard

import (
	"errors"
	"fmt"
	"slices"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd/auth"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/version"
)

// fallbackMsgs mirror auth.ExchangeToken's errors for 404 and 401; servers without the exchange endpoint answer 401
// because they reject the unknown onboarding token before routing, so both mean "maybe unsupported".
var fallbackMsgs = []string{
	"this TeamCity server doesn't support onboarding tokens",
	"onboarding token is expired or invalid",
}

// accessToken exchanges the onboarding token, or uses the saved login when none is given or the exchange is unsupported or rejected.
func accessToken(f *cmdutil.Factory, serverURL, onboardingToken string) (string, error) {
	if onboardingToken == "" {
		return savedLogin(f, serverURL, "")
	}

	token, exchangeErr := auth.ExchangeToken(f, serverURL, onboardingToken)
	ve, ok := errors.AsType[*api.ValidationError](exchangeErr)
	if !ok || !slices.Contains(fallbackMsgs, ve.Error()) {
		return token, exchangeErr
	}
	saved, err := savedLogin(f, serverURL, fmt.Sprintf("couldn't exchange the onboarding token (%s) and ", ve.Error()))
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
