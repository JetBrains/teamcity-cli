package auth

import "github.com/JetBrains/teamcity-cli/internal/cmdutil"

// ExchangeToken trades a short-lived onboarding token for a TeamCity access token and stores it like `auth login`.
func ExchangeToken(_ *cmdutil.Factory, _, token string) (string, error) {
	// TODO: stub, call exchange endpoint
	// TODO: persist like finishTokenLogin
	return token, nil
}
