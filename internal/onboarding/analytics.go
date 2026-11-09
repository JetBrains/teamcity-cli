package onboarding

import (
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
)

// TrackPipelineCreated tracks a pipeline that `pipeline create` created inside the onboarding; outside it, it does nothing.
func TrackPipelineCreated(f *cmdutil.Factory) {
	rt, ok := Current()
	if !ok {
		return
	}
	f.Analytics.Track(analytics.GroupOnboarding, analytics.EventPipelineCreated, map[string]any{
		"repository": string(rt),
	})
}

// TrackBuildQueued tracks a build that `run start` queued inside the onboarding, and remembers it to track its start and finish later; outside the onboarding, it does nothing.
func TrackBuildQueued(f *cmdutil.Factory, buildID int) {
	rt, ok := Current()
	if !ok {
		return
	}
	f.Analytics.Track(analytics.GroupOnboarding, analytics.EventBuildQueued, map[string]any{
		"repository": string(rt),
	})
	RememberBuild(buildID, rt)
}
