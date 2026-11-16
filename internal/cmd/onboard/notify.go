package onboard

import (
	"context"

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
	endDeclined
	endSkillInstallFailed
	endAgentStartFailed
	endAgentInterrupted
	endAgentError
	endRerunDeclined
	endAgentExited
)

// handoverStatus maps where the hand-over stopped to the status the server report sends.
func handoverStatus(end handoverEnd) api.OnboardingHandoverStatus {
	switch end {
	case endNoCheckout:
		return api.OnboardingStatusNoCheckout
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

// notifyInBackground sends reports the onboarding doesn't wait for; tests replace it with onboarding.Notify to see them when the call returns.
var notifyInBackground = onboarding.NotifyInBackground

// reportedAgent is a's AI agent enum value.
func reportedAgent(a agent) api.OnboardingAgent {
	return api.OnboardingAgent(analytics.NormalizeAIAgent(a.name))
}

// reportedAgents lists the detected agents by their AI agent enum value, or none.
func reportedAgents(agents []agent) []api.OnboardingAgent {
	if len(agents) == 0 {
		return []api.OnboardingAgent{api.OnboardingAgentNone}
	}
	out := make([]api.OnboardingAgent, len(agents))
	for i, a := range agents {
		out[i] = reportedAgent(a)
	}
	return out
}

func notifyLoginCompleted(f *cmdutil.Factory, client api.ClientInterface) {
	notifyInBackground(f.Context(), f.Printer.Debug, "login-completed", client.OnboardingLoginCompleted)
}

// notifyAgentsDetected reports the detected agents, waiting since teamcity onboard exits right after when there are none.
func notifyAgentsDetected(f *cmdutil.Factory, client api.ClientInterface, agents []agent) {
	onboarding.Notify(f.Context(), f.Printer.Debug, "agents-detected", func(ctx context.Context) error {
		return client.OnboardingAgentsDetected(ctx, api.OnboardingAgentsDetectedRequest{Agents: reportedAgents(agents)})
	})
}

func notifySandboxChecked(f *cmdutil.Factory, client api.ClientInterface, ag agent, status api.OnboardingSandboxStatus) {
	notifyInBackground(f.Context(), f.Printer.Debug, "sandbox-checked", func(ctx context.Context) error {
		return client.OnboardingSandboxChecked(ctx, api.OnboardingSandboxCheckedRequest{Agent: reportedAgent(ag), Status: status})
	})
}

// notifyHandoverStarted reports that ag is about to be launched with the onboarding prompt; reruns don't report it again.
func notifyHandoverStarted(f *cmdutil.Factory, client api.ClientInterface, rt api.OnboardingRepositoryType, ag agent) {
	notifyInBackground(f.Context(), f.Printer.Debug, "handover-started", func(ctx context.Context) error {
		return client.OnboardingHandoverStarted(ctx, api.OnboardingHandoverStartedRequest{RepositoryType: rt, Agent: reportedAgent(ag)})
	})
}

func notifyHandoverFinished(f *cmdutil.Factory, client api.ClientInterface, rt api.OnboardingRepositoryType, status api.OnboardingHandoverStatus) {
	onboarding.Notify(f.Context(), f.Printer.Debug, "handover-finished", func(ctx context.Context) error {
		return client.OnboardingHandoverFinished(ctx, api.OnboardingHandoverFinishedRequest{RepositoryType: rt, Status: status})
	})
}
