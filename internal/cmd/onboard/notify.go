package onboard

import (
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/onboarding"
)

// repositoryType maps where the repository comes from to the type the onboarding reports.
func repositoryType(source repoSource) api.OnboardingRepositoryType {
	if source == sourceSample {
		return api.OnboardingRepositoryDemo
	}
	return api.OnboardingRepositoryOwn
}

// notifyInBackground sends reports the onboarding doesn't wait for; tests replace it with onboarding.Notify to see them when the call returns.
var notifyInBackground = onboarding.NotifyInBackground

func notifyLoginCompleted(f *cmdutil.Factory, client api.ClientInterface) {
	notifyInBackground(f.Context(), f.Printer.Debug, "login-completed", client.OnboardingLoginCompleted)
}
