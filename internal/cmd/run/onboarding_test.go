package run_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
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

// rememberedBuilds reads the onboarding builds state file under xdg; nil when there is none.
func rememberedBuilds(t *testing.T, xdg string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(xdg, "tc", ".analytics", "onboarding-builds.json"))
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	builds := []map[string]any{}
	require.NoError(t, json.Unmarshal(data, &builds))
	return builds
}

func TestRunStartInsideOnboardingRemembersBuild(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	xdg := cmdtest.EnableAnalytics(t)
	t.Setenv(onboarding.EnvOnboarding, "demo")
	ts.HandleOnboarding(http.StatusNoContent)

	cmdtest.RunCmdWithFactory(t, ts.Factory, "run", "start", testJob)

	builds := rememberedBuilds(t, xdg)
	require.Len(t, builds, 1)
	assert.EqualValues(t, 100, builds[0]["id"])
	assert.Equal(t, "demo", builds[0]["repository"])
	assert.Equal(t, false, builds[0]["started"])
}

func TestRunStartOutsideOnboardingRemembersNothing(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	xdg := cmdtest.EnableAnalytics(t)
	t.Setenv(onboarding.EnvOnboarding, "")

	cmdtest.RunCmdWithFactory(t, ts.Factory, "run", "start", testJob)
	assert.Nil(t, rememberedBuilds(t, xdg))
}

func TestRunViewReportsRememberedBuild(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	xdg := cmdtest.EnableAnalytics(t)
	t.Setenv(onboarding.EnvOnboarding, "own")
	ts.HandleOnboarding(http.StatusNoContent)
	cmdtest.RunCmdWithFactory(t, ts.Factory, "run", "start", testJob)
	require.Len(t, rememberedBuilds(t, xdg), 1)

	var build atomic.Pointer[api.Build]
	build.Store(&api.Build{ID: 100, Number: "100", BuildTypeID: testJob, State: "running", Status: "SUCCESS"})
	ts.Handle("GET /app/rest/builds/id:100", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, build.Load())
	})
	t.Setenv(onboarding.EnvOnboarding, "") // later checks may run outside the onboarding

	cmdtest.RunCmdWithFactory(t, ts.Factory, "run", "view", "100", "--json")
	builds := rememberedBuilds(t, xdg)
	require.Len(t, builds, 1)
	assert.Equal(t, true, builds[0]["started"])

	cmdtest.RunCmdWithFactory(t, ts.Factory, "run", "view", "999", "--json") // a build that isn't remembered changes nothing
	assert.Equal(t, builds, rememberedBuilds(t, xdg))

	build.Store(&api.Build{ID: 100, Number: "100", BuildTypeID: testJob, State: "finished", Status: "FAILURE"})
	cmdtest.RunCmdWithFactory(t, ts.Factory, "run", "view", "100", "--json")
	assert.Empty(t, rememberedBuilds(t, xdg), "forgotten after build.finished")
}
