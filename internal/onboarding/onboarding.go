// Package onboarding passes the `teamcity onboard` context to the commands its AI agent runs, and reports onboarding steps to the server.
package onboarding

import (
	"context"
	"os"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/config"
)

// EnvOnboarding carries the repository type (own or demo) from `teamcity onboard` to the commands its AI agent runs.
const EnvOnboarding = "TEAMCITY_ONBOARDING"

// Current returns the repository type when running inside `teamcity onboard`; any other value of EnvOnboarding is ignored.
func Current() (api.OnboardingRepositoryType, bool) {
	switch rt := api.OnboardingRepositoryType(os.Getenv(EnvOnboarding)); rt {
	case api.OnboardingRepositoryOwn, api.OnboardingRepositoryDemo:
		return rt, true
	}
	return "", false
}

// Env returns the EnvOnboarding entry for the agent's environment.
func Env(rt api.OnboardingRepositoryType) string {
	return EnvOnboarding + "=" + string(rt)
}

// Enabled reports whether onboarding reports may be sent and recorded; not when CLI analytics is opted out.
func Enabled() bool {
	enabled, _ := analytics.IsEnabled(config.IsAnalyticsEnabled())
	return enabled
}

// Notify runs call, which reports one onboarding step to the server, unless CLI analytics is opted out; a failure is only logged at debug level.
func Notify(ctx context.Context, debug func(string, ...any), endpoint string, call func(context.Context) error) {
	if !Enabled() {
		return
	}
	// Not canceled with ctx, so an interrupted onboarding still reports how it ended; the client bounds the call with a short timeout.
	if err := call(context.WithoutCancel(ctx)); err != nil {
		debug("onboarding: %s not sent: %v", endpoint, err)
	}
}
