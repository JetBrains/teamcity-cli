package onboarding

import (
	"context"
	"errors"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestCurrent(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		want   api.OnboardingRepositoryType
		wantOK bool
	}{
		{"own", "own", api.OnboardingRepositoryOwn, true},
		{"demo", "demo", api.OnboardingRepositoryDemo, true},
		{"unset", "", "", false},
		{"unknown", "sample", "", false},
		{"not normalized", " Own", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvOnboarding, tc.value)
			got, ok := Current()
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantOK, ok)
		})
	}
}

func TestEnv(t *testing.T) {
	assert.Equal(t, "TEAMCITY_ONBOARDING=demo", Env(api.OnboardingRepositoryDemo))
}

// enableAnalytics clears the analytics opt-outs for the test.
func enableAnalytics(t *testing.T) {
	t.Helper()
	t.Setenv(analytics.EnvDoNotTrack, "")
	t.Setenv(analytics.EnvAnalytics, "")
	config.ResetForTest()
}

func TestNotify(t *testing.T) {
	t.Run("sends with a context that outlives cancellation", func(t *testing.T) {
		enableAnalytics(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		calls := 0
		Notify(ctx, t.Logf, "build-queued", func(ctx context.Context) error {
			calls++
			assert.NoError(t, ctx.Err())
			return nil
		})
		assert.Equal(t, 1, calls)
	})

	t.Run("logs a failure at debug level only", func(t *testing.T) {
		enableAnalytics(t)
		var logged []string
		debug := func(format string, args ...any) { logged = append(logged, format) }
		Notify(t.Context(), debug, "build-queued", func(context.Context) error { return errors.New("404") })
		assert.Len(t, logged, 1)
	})

	for name, env := range map[string][2]string{
		"DO_NOT_TRACK":         {analytics.EnvDoNotTrack, "1"},
		"TEAMCITY_ANALYTICS=0": {analytics.EnvAnalytics, "0"},
	} {
		t.Run("skipped when opted out with "+name, func(t *testing.T) {
			enableAnalytics(t)
			t.Setenv(env[0], env[1])
			Notify(t.Context(), t.Logf, "build-queued", func(context.Context) error {
				t.Error("no call expected when opted out")
				return nil
			})
		})
	}
}
