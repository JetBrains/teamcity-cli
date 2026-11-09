package run

import (
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/onboarding"
)

// trackOnboardingBuild tracks build.started and build.finished, once each, for a build queued inside the onboarding; other builds change nothing.
func trackOnboardingBuild(f *cmdutil.Factory, b *api.Build) {
	ev, ok := onboarding.ObserveBuild(b.ID, b.State)
	if !ok {
		return
	}
	if ev.Started {
		f.Analytics.Track(analytics.GroupOnboarding, analytics.EventBuildStarted, map[string]any{"repository": ev.Repository})
	}
	if ev.Finished {
		f.Analytics.Track(analytics.GroupOnboarding, analytics.EventBuildFinished, onboardingBuildFinishedFields(b, ev.Repository))
	}
}

func onboardingBuildFinishedFields(b *api.Build, repository string) map[string]any {
	return map[string]any{"repository": repository, "final_status": buildFinalStatus(b.Status)}
}
