package onboard

import (
	"slices"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
)

// detectedAgentFields maps the detected agents to one has_<agent> boolean per supported agent, from its AI agent enum value.
func detectedAgentFields(agents []agent) map[string]any {
	fields := make(map[string]any, len(supportedAgents))
	for _, s := range supportedAgents {
		fields["has_"+analytics.NormalizeAIAgent(s.name)] = slices.ContainsFunc(agents, func(a agent) bool { return a.name == s.name })
	}
	return fields
}

func trackAgentsDetected(f *cmdutil.Factory, agents []agent) {
	f.Analytics.Track(analytics.GroupOnboarding, analytics.EventAgentsDetected, detectedAgentFields(agents))
}

// trackHandoverStarted tracks that ag is about to be launched with the onboarding prompt; reruns don't track it again.
func trackHandoverStarted(f *cmdutil.Factory, rt api.OnboardingRepositoryType, ag agent) {
	f.Analytics.Track(analytics.GroupOnboarding, analytics.EventHandoverStarted, map[string]any{
		"repository": string(rt),
		"agent":      analytics.NormalizeAIAgent(ag.name),
	})
}

// trackHandoverFinished tracks how the hand-over ended; the outcome takes the status the server report sends.
func trackHandoverFinished(f *cmdutil.Factory, rt api.OnboardingRepositoryType, status api.OnboardingHandoverStatus) {
	f.Analytics.Track(analytics.GroupOnboarding, analytics.EventHandoverFinished, map[string]any{
		"repository": string(rt),
		"outcome":    string(status),
	})
}
