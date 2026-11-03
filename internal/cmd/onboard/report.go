package onboard

import (
	"context"
	"slices"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
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

// handoverEnd is where handOverToAgent stopped.
type handoverEnd int

const (
	endFailed handoverEnd = iota // any error not covered below
	endNoCheckout
	endNoAgent
	endDeclined
	endSkillInstallFailed
	endAgentStartFailed
	endAgentInterrupted
	endAgentError
	endRerunDeclined
	endAgentExited
)

// handoverStatus maps where the hand-over stopped to its reported status, shared by the server report and the analytics outcome.
func handoverStatus(end handoverEnd) api.OnboardingHandoverStatus {
	switch end {
	case endNoCheckout:
		return api.OnboardingStatusNoCheckout
	case endNoAgent:
		return api.OnboardingStatusNoAgent
	case endDeclined:
		return api.OnboardingStatusHandoverDeclined
	case endSkillInstallFailed:
		return api.OnboardingStatusSkillInstallFailure
	case endAgentStartFailed:
		return api.OnboardingStatusAgentFailedToStart
	case endAgentInterrupted:
		return api.OnboardingStatusAgentInterrupted
	case endAgentError:
		return api.OnboardingStatusAgentError
	case endRerunDeclined:
		return api.OnboardingStatusHandoverRerunDeclined
	case endAgentExited:
		return api.OnboardingStatusSuccess
	default:
		return api.OnboardingStatusError
	}
}

// reportedAgents lists the detected agents by their AI agent enum value, or none.
func reportedAgents(agents []agent) []api.OnboardingAgent {
	if len(agents) == 0 {
		return []api.OnboardingAgent{api.OnboardingAgentNone}
	}
	out := make([]api.OnboardingAgent, len(agents))
	for i, a := range agents {
		out[i] = api.OnboardingAgent(analytics.NormalizeAIAgent(a.name))
	}
	return out
}

func reportLoginCompleted(f *cmdutil.Factory, client api.ClientInterface) {
	onboarding.Notify(f.Context(), f.Printer.Debug, "login-completed", client.OnboardingLoginCompleted)
}

// detectedAgentFields maps the detected agents to one has_<agent> boolean per supported agent, from its AI agent enum value.
func detectedAgentFields(agents []agent) map[string]any {
	fields := make(map[string]any, len(supportedAgents))
	for _, s := range supportedAgents {
		fields["has_"+analytics.NormalizeAIAgent(s.name)] = slices.ContainsFunc(agents, func(a agent) bool { return a.name == s.name })
	}
	return fields
}

func reportAgentsDetected(f *cmdutil.Factory, client api.ClientInterface, agents []agent) {
	f.Analytics.Track(analytics.GroupOnboarding, analytics.EventAgentsDetected, detectedAgentFields(agents))
	onboarding.Notify(f.Context(), f.Printer.Debug, "agents-detected", func(ctx context.Context) error {
		return client.OnboardingAgentsDetected(ctx, api.OnboardingAgentsDetectedRequest{Agents: reportedAgents(agents)})
	})
}

func reportHandoverStarted(f *cmdutil.Factory, client api.ClientInterface, rt api.OnboardingRepositoryType) {
	f.Analytics.Track(analytics.GroupOnboarding, analytics.EventHandoverStarted, map[string]any{"repository": string(rt)})
	onboarding.Notify(f.Context(), f.Printer.Debug, "handover-started", func(ctx context.Context) error {
		return client.OnboardingHandoverStarted(ctx, api.OnboardingHandoverStartedRequest{RepositoryType: rt})
	})
}

func reportHandoverFinished(f *cmdutil.Factory, client api.ClientInterface, rt api.OnboardingRepositoryType, status api.OnboardingHandoverStatus) {
	f.Analytics.Track(analytics.GroupOnboarding, analytics.EventHandoverFinished, map[string]any{
		"repository": string(rt),
		"outcome":    string(status),
	})
	onboarding.Notify(f.Context(), f.Printer.Debug, "handover-finished", func(ctx context.Context) error {
		return client.OnboardingHandoverFinished(ctx, api.OnboardingHandoverFinishedRequest{RepositoryType: rt, Status: status})
	})
}
