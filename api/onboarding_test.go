package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// onboardingCalls covers each onboarding endpoint with the body the contract expects ("" for none).
var onboardingCalls = []struct {
	endpoint string
	call     func(context.Context, *Client) error
	body     string
}{
	{"login-completed", func(ctx context.Context, c *Client) error {
		return c.OnboardingLoginCompleted(ctx)
	}, ""},
	{"agents-detected", func(ctx context.Context, c *Client) error {
		return c.OnboardingAgentsDetected(ctx, OnboardingAgentsDetectedRequest{Agents: []OnboardingAgent{OnboardingAgentClaudeCode, OnboardingAgentJunie}})
	}, `{"agents":["claude_code","junie"]}`},
	{"handover-started", func(ctx context.Context, c *Client) error {
		return c.OnboardingHandoverStarted(ctx, OnboardingHandoverStartedRequest{RepositoryType: OnboardingRepositoryDemo})
	}, `{"repositoryType":"demo"}`},
	{"handover-finished", func(ctx context.Context, c *Client) error {
		return c.OnboardingHandoverFinished(ctx, OnboardingHandoverFinishedRequest{RepositoryType: OnboardingRepositoryOwn, Status: OnboardingStatusHandoverRerunDeclined})
	}, `{"repositoryType":"own","status":"handover_rerun_declined"}`},
	{"pipeline-created", func(ctx context.Context, c *Client) error {
		return c.OnboardingPipelineCreated(ctx, OnboardingPipelineCreatedRequest{RepositoryType: OnboardingRepositoryOwn, PipelineID: "Instance_App_CI"})
	}, `{"repositoryType":"own","pipelineId":"Instance_App_CI"}`},
	{"build-queued", func(ctx context.Context, c *Client) error {
		return c.OnboardingBuildQueued(ctx, OnboardingBuildQueuedRequest{RepositoryType: OnboardingRepositoryDemo, BuildID: 4242})
	}, `{"repositoryType":"demo","buildId":4242}`},
}

func TestOnboardingEndpoints(t *testing.T) {
	t.Parallel()
	for _, tc := range onboardingCalls {
		t.Run(tc.endpoint, func(t *testing.T) {
			t.Parallel()
			client := NewClient("https://teamcity.example.com", "access-token")
			client.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/app/fus/onboarding/cli/"+tc.endpoint, r.URL.Path)
				assert.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				deadline, ok := r.Context().Deadline()
				assert.True(t, ok, "request must have a deadline")
				assert.WithinDuration(t, time.Now().Add(onboardingTimeout), deadline, time.Second)
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				if tc.body == "" {
					assert.Empty(t, body)
				} else {
					assert.JSONEq(t, tc.body, string(body))
				}
				return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
			})

			require.NoError(t, tc.call(t.Context(), client))
		})
	}
}

func TestOnboardingEndpointsFailOnce(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusNotFound, http.StatusBadRequest, http.StatusServiceUnavailable} {
		for _, tc := range onboardingCalls {
			t.Run(http.StatusText(status)+"/"+tc.endpoint, func(t *testing.T) {
				t.Parallel()
				calls := 0
				client := NewClient("https://teamcity.example.com", "access-token")
				client.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}, nil
				})

				err := tc.call(t.Context(), client)
				require.Error(t, err)
				assert.Equal(t, 1, calls, "no retry")
				if status == http.StatusNotFound {
					_, ok := errors.AsType[*NotFoundError](err)
					assert.True(t, ok, "want *NotFoundError, got %T: %v", err, err)
				}
			})
		}
	}
}

func TestOnboardingEndpointTimeout(t *testing.T) {
	t.Parallel()
	calls := 0
	client := NewClient("https://teamcity.example.com", "access-token")
	client.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	err := client.OnboardingBuildQueued(ctx, OnboardingBuildQueuedRequest{RepositoryType: OnboardingRepositoryOwn, BuildID: 1})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, calls, "no retry")
}
