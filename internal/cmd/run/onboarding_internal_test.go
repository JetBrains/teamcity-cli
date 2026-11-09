package run

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/stretchr/testify/assert"
)

func TestOnboardingBuildFinishedFields(t *testing.T) {
	t.Parallel()
	for status, want := range map[string]string{
		"SUCCESS": analytics.BuildStatusSuccess,
		"UNKNOWN": analytics.BuildStatusCanceled, // a canceled build
	} {
		got := onboardingBuildFinishedFields(&api.Build{ID: 42, State: "finished", Status: status}, analytics.OnboardingRepositoryDemo)
		assert.Equal(t, map[string]any{"repository": "demo", "final_status": want}, got, status)
	}
}
