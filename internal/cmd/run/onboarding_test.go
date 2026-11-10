package run_test

import (
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/onboarding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStartInsideOnboardingReportsBuild(t *testing.T) {
	for name, args := range map[string][]string{
		"plain": {"run", "start", testJob},
		"json":  {"run", "start", testJob, "--json"},
	} {
		t.Run(name, func(t *testing.T) {
			ts := cmdtest.SetupMockClient(t)
			cmdtest.EnableAnalytics(t)
			t.Setenv(onboarding.EnvOnboarding, "own")
			rec := ts.HandleOnboarding(http.StatusNoContent)

			cmdtest.RunCmdWithFactory(t, ts.Factory, args...)

			calls := rec.Calls()
			require.Len(t, calls, 1)
			assert.Equal(t, "build-queued", calls[0].Endpoint)
			assert.JSONEq(t, `{"repositoryType":"own","buildId":100}`, calls[0].Body)
		})
	}
}

func TestRunStartOutsideOnboardingReportsNothing(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	cmdtest.EnableAnalytics(t)
	t.Setenv(onboarding.EnvOnboarding, "")
	rec := ts.HandleOnboarding(http.StatusNoContent)

	cmdtest.RunCmdWithFactory(t, ts.Factory, "run", "start", testJob)
	assert.Empty(t, rec.Calls())
}

func TestRunStartInsideOnboardingOptedOut(t *testing.T) {
	ts := cmdtest.SetupMockClient(t) // sets DO_NOT_TRACK=1
	t.Setenv(onboarding.EnvOnboarding, "demo")
	rec := ts.HandleOnboarding(http.StatusNoContent)

	cmdtest.RunCmdWithFactory(t, ts.Factory, "run", "start", testJob)
	assert.Empty(t, rec.Calls())
}

func TestRunStartInsideOnboardingIgnoresOlderServer(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	cmdtest.EnableAnalytics(t)
	t.Setenv(onboarding.EnvOnboarding, "own")
	rec := ts.HandleOnboarding(http.StatusNoContent)

	want := cmdtest.CaptureOutput(t, ts.Factory, "run", "start", testJob)
	rec.Respond(http.StatusNotFound)
	got := cmdtest.CaptureOutput(t, ts.Factory, "run", "start", testJob)

	assert.Equal(t, want, got)
	assert.Len(t, rec.Calls(), 2)
}
