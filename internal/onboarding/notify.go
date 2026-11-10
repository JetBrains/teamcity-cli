package onboarding

import (
	"context"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
)

// NotifyPipelineCreated reports a pipeline that `pipeline create` created inside the onboarding to the server; outside it, it does nothing.
func NotifyPipelineCreated(f *cmdutil.Factory, client api.ClientInterface, pipelineID string) {
	rt, ok := Current()
	if !ok {
		return
	}
	Notify(f.Context(), f.Printer.Debug, "pipeline-created", func(ctx context.Context) error {
		return client.OnboardingPipelineCreated(ctx, api.OnboardingPipelineCreatedRequest{RepositoryType: rt, PipelineID: pipelineID})
	})
}

// NotifyBuildQueued reports a build that `run start` queued inside the onboarding to the server; outside it, it does nothing.
func NotifyBuildQueued(f *cmdutil.Factory, client api.ClientInterface, buildID int) {
	rt, ok := Current()
	if !ok {
		return
	}
	Notify(f.Context(), f.Printer.Debug, "build-queued", func(ctx context.Context) error {
		return client.OnboardingBuildQueued(ctx, api.OnboardingBuildQueuedRequest{RepositoryType: rt, BuildID: buildID})
	})
}
