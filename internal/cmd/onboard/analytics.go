package onboard

import (
	"errors"
	"slices"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/charmbracelet/huh"
)

// detectedAgentFields maps the detected agents to one has_<agent> boolean per supported agent, from its AI agent enum value.
func detectedAgentFields(agents []agent) map[string]any {
	fields := make(map[string]any, len(supportedAgents))
	for _, s := range supportedAgents {
		fields["has_"+analytics.NormalizeAIAgent(s.name)] = slices.ContainsFunc(agents, func(a agent) bool { return a.name == s.name })
	}
	return fields
}

// track records an onboarding event; tests replace it to see the events.
var track = func(f *cmdutil.Factory, event string, data map[string]any) {
	f.Analytics.Track(analytics.GroupOnboarding, event, data)
}

func trackAgentsDetected(f *cmdutil.Factory, agents []agent) {
	track(f, analytics.EventAgentsDetected, detectedAgentFields(agents))
}

// trackSandboxChecked tracks how the step that lets teamcity run outside ag's sandbox ended.
func trackSandboxChecked(f *cmdutil.Factory, ag agent, status api.OnboardingSandboxStatus) {
	track(f, analytics.EventSandboxChecked, map[string]any{
		"agent":  analytics.NormalizeAIAgent(ag.name),
		"status": string(status),
	})
}

// trackHandoverStarted tracks that ag is about to be launched with the onboarding prompt; reruns don't track it again.
func trackHandoverStarted(f *cmdutil.Factory, rt api.OnboardingRepositoryType, ag agent) {
	track(f, analytics.EventHandoverStarted, map[string]any{
		"repository": string(rt),
		"agent":      analytics.NormalizeAIAgent(ag.name),
	})
}

// trackHandoverFinished tracks how the hand-over ended; the outcome takes the status the server report sends.
func trackHandoverFinished(f *cmdutil.Factory, rt api.OnboardingRepositoryType, status api.OnboardingHandoverStatus) {
	track(f, analytics.EventHandoverFinished, map[string]any{
		"repository": string(rt),
		"outcome":    string(status),
	})
}

// promptEnd is declined when the user aborted a prompt (Ctrl+C), and endFailed for any other prompt failure.
func promptEnd(err error, declined handoverEnd) handoverEnd {
	if errors.Is(err, huh.ErrUserAborted) {
		return declined
	}
	return endFailed
}
