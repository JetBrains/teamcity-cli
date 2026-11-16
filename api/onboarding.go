package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OnboardingRepositoryType is the repository `teamcity onboard` set up: the user's own or the sample.
type OnboardingRepositoryType string

const (
	OnboardingRepositoryOwn  OnboardingRepositoryType = "own"
	OnboardingRepositoryDemo OnboardingRepositoryType = "demo"
)

type OnboardingPipelineCreatedRequest struct {
	RepositoryType OnboardingRepositoryType `json:"repositoryType"`
	PipelineID     string                   `json:"pipelineId"`
}

type OnboardingBuildQueuedRequest struct {
	RepositoryType OnboardingRepositoryType `json:"repositoryType"`
	BuildID        int                      `json:"buildId"`
}

// onboardingTimeout bounds each onboarding report, so a slow server can't stall the onboarding.
const onboardingTimeout = 3 * time.Second

// OnboardingLoginCompleted reports that `teamcity onboard` logged in.
func (c *Client) OnboardingLoginCompleted(ctx context.Context) error {
	return c.postOnboarding(ctx, "login-completed", nil)
}

// OnboardingPipelineCreated reports a pipeline created inside the onboarding.
func (c *Client) OnboardingPipelineCreated(ctx context.Context, req OnboardingPipelineCreatedRequest) error {
	return c.postOnboarding(ctx, "pipeline-created", req)
}

// OnboardingBuildQueued reports a build queued inside the onboarding.
func (c *Client) OnboardingBuildQueued(ctx context.Context, req OnboardingBuildQueuedRequest) error {
	return c.postOnboarding(ctx, "build-queued", req)
}

// postOnboarding posts req (no body when nil) to an onboarding endpoint once, within onboardingTimeout; a 404 means an older server.
func (c *Client) postOnboarding(ctx context.Context, endpoint string, req any) error {
	ctx, cancel := context.WithTimeout(ctx, onboardingTimeout)
	defer cancel()
	var body io.Reader = http.NoBody // still sends Content-Type: application/json
	if req != nil {
		data, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	return c.doNoContent(ctx, http.MethodPost, "/app/fus/onboarding/cli/"+endpoint, body, "")
}
