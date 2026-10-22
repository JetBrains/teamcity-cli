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

// accessToken exchanges the onboarding token, falling back to a working saved login when the exchange is unsupported or rejected.
func accessToken(f *cmdutil.Factory, serverURL, onboardingToken string) (string, error) {
	token, exchangeErr := auth.ExchangeToken(f, serverURL, onboardingToken)
	ve, ok := errors.AsType[*api.ValidationError](exchangeErr)
	if !ok || !slices.Contains(fallbackMsgs, ve.Error()) {
		return token, exchangeErr
	}

	saved, _, err := config.GetTokenForServer(serverURL)
	if err != nil {
		return "", fmt.Errorf("read token for %s: %w", serverURL, err)
	}
	if saved == "" || !loginWorks(f, serverURL, saved) {
		return "", api.Validation(
			fmt.Sprintf("couldn't exchange the onboarding token (%s) and you're not logged in to %s", ve.Error(), serverURL),
			fmt.Sprintf("Generate a new onboarding token, or run teamcity auth login -s %s, then rerun teamcity onboard", serverURL),
		)
	}
	f.Printer.Warn("Couldn't exchange the onboarding token; using your existing login for %s", serverURL)
	return saved, nil
}

// loginWorks reports whether token authenticates against serverURL.
func loginWorks(f *cmdutil.Factory, serverURL, token string) bool {
	client := api.NewClient(serverURL, token, api.WithDebugFunc(f.Printer.Debug), api.WithVersion(version.String())).WithContext(f.Context())
	_, err := client.GetCurrentUser()
	return err == nil
}
