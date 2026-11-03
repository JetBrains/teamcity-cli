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

// OnboardingAgent is a detected AI agent, or OnboardingAgentNone when there is none.
type OnboardingAgent string

const (
	OnboardingAgentClaudeCode OnboardingAgent = "claude_code"
	OnboardingAgentCodex      OnboardingAgent = "codex"
	OnboardingAgentJunie      OnboardingAgent = "junie"
	OnboardingAgentNone       OnboardingAgent = "none"
)

// OnboardingHandoverStatus is how the hand-over to the AI agent ended.
type OnboardingHandoverStatus string

const (
	OnboardingStatusSuccess               OnboardingHandoverStatus = "success"
	OnboardingStatusNoCheckout            OnboardingHandoverStatus = "no_checkout"
	OnboardingStatusNoAgent               OnboardingHandoverStatus = "no_agent"
	OnboardingStatusHandoverDeclined      OnboardingHandoverStatus = "handover_declined"
	OnboardingStatusSkillInstallFailure   OnboardingHandoverStatus = "skill_install_failure"
	OnboardingStatusAgentFailedToStart    OnboardingHandoverStatus = "agent_failed_to_start"
	OnboardingStatusAgentInterrupted      OnboardingHandoverStatus = "agent_interrupted"
	OnboardingStatusAgentError            OnboardingHandoverStatus = "agent_error"
	OnboardingStatusHandoverRerunDeclined OnboardingHandoverStatus = "handover_rerun_declined"
	OnboardingStatusError                 OnboardingHandoverStatus = "error"
)

type OnboardingAgentsDetectedRequest struct {
	Agents []OnboardingAgent `json:"agents"`
}

type OnboardingHandoverStartedRequest struct {
	RepositoryType OnboardingRepositoryType `json:"repositoryType"`
}

type OnboardingHandoverFinishedRequest struct {
	RepositoryType OnboardingRepositoryType `json:"repositoryType"`
	Status         OnboardingHandoverStatus `json:"status"`
}

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

// OnboardingAgentsDetected reports the AI agents `teamcity onboard` found.
func (c *Client) OnboardingAgentsDetected(ctx context.Context, req OnboardingAgentsDetectedRequest) error {
	return c.postOnboarding(ctx, "agents-detected", req)
}

// OnboardingHandoverStarted reports that `teamcity onboard` began the hand-over to an AI agent.
func (c *Client) OnboardingHandoverStarted(ctx context.Context, req OnboardingHandoverStartedRequest) error {
	return c.postOnboarding(ctx, "handover-started", req)
}

// OnboardingHandoverFinished reports how the hand-over to the AI agent ended.
func (c *Client) OnboardingHandoverFinished(ctx context.Context, req OnboardingHandoverFinishedRequest) error {
	return c.postOnboarding(ctx, "handover-finished", req)
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
