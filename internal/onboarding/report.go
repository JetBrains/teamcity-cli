package onboarding

import (
	"context"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
)

// PipelineCreated reports a pipeline that `pipeline create` created inside the onboarding; outside it, it does nothing.
func PipelineCreated(f *cmdutil.Factory, client api.ClientInterface, pipelineID string) {
	rt, ok := Current()
	if !ok {
		return
	}
	Notify(f.Context(), f.Printer.Debug, "pipeline-created", func(ctx context.Context) error {
		return client.OnboardingPipelineCreated(ctx, api.OnboardingPipelineCreatedRequest{RepositoryType: rt, PipelineID: pipelineID})
	})
}

// BuildQueued reports a build that `run start` queued inside the onboarding; outside it, it does nothing.
func BuildQueued(f *cmdutil.Factory, client api.ClientInterface, buildID int) {
	rt, ok := Current()
	if !ok {
		return
	}
	Notify(f.Context(), f.Printer.Debug, "build-queued", func(ctx context.Context) error {
		return client.OnboardingBuildQueued(ctx, api.OnboardingBuildQueuedRequest{RepositoryType: rt, BuildID: buildID})
	})
}
