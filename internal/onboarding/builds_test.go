package onboarding

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func buildsFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), buildsFileName)
}

func TestRememberBuildRoundTrip(t *testing.T) {
	path := buildsFile(t)
	require.NoError(t, rememberBuildAt(path, t0, 42, "demo"))
	require.NoError(t, rememberBuildAt(path, t0.Add(time.Minute), 42, "own")) // a repeat keeps the first entry

	assert.Equal(t, []rememberedBuild{{ID: 42, Repository: "demo", AddedAt: t0}}, readBuilds(path))
}

func TestObserveBuildReportsOncePerBuild(t *testing.T) {
	path := buildsFile(t)
	require.NoError(t, rememberBuildAt(path, t0, 42, "own"))

	ev, ok := observeBuildAt(path, t0, 42, "running")
	assert.True(t, ok)
	assert.Equal(t, BuildEvents{Repository: "own", Started: true}, ev)

	ev, ok = observeBuildAt(path, t0, 42, "running")
	assert.True(t, ok)
	assert.Equal(t, BuildEvents{Repository: "own"}, ev, "build.started already recorded")

	ev, ok = observeBuildAt(path, t0, 42, "finished")
	assert.True(t, ok)
	assert.Equal(t, BuildEvents{Repository: "own", Finished: true}, ev)

	_, ok = observeBuildAt(path, t0, 42, "finished")
	assert.False(t, ok, "forgotten after build.finished")
	assert.Empty(t, readBuilds(path))
}

func TestObserveBuildFirstSeenFinished(t *testing.T) {
	path := buildsFile(t)
	require.NoError(t, rememberBuildAt(path, t0, 42, "demo"))

	ev, ok := observeBuildAt(path, t0, 42, "finished")
	assert.True(t, ok)
	assert.Equal(t, BuildEvents{Repository: "demo", Started: true, Finished: true}, ev)

	_, ok = observeBuildAt(path, t0, 42, "finished")
	assert.False(t, ok)
}

func TestObserveBuildQueuedReportsNothing(t *testing.T) {
	path := buildsFile(t)
	require.NoError(t, rememberBuildAt(path, t0, 42, "own"))

	_, ok := observeBuildAt(path, t0, 42, "queued")
	assert.False(t, ok)
	assert.Equal(t, []rememberedBuild{{ID: 42, Repository: "own", AddedAt: t0}}, readBuilds(path))
}

func TestObserveBuildNotRemembered(t *testing.T) {
	path := buildsFile(t)
	require.NoError(t, rememberBuildAt(path, t0, 42, "own"))
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)

	_, ok := observeBuildAt(path, t0.Add(time.Hour), 7, "finished")
	assert.False(t, ok)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	info2, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, info.ModTime(), info2.ModTime(), "file not rewritten")
}

func TestRememberBuildExpiryAndCap(t *testing.T) {
	t.Run("expiry", func(t *testing.T) {
		path := buildsFile(t)
		require.NoError(t, rememberBuildAt(path, t0, 1, "own"))
		require.NoError(t, rememberBuildAt(path, t0.Add(buildsMaxAge+time.Hour), 2, "own"))

		builds := readBuilds(path)
		require.Len(t, builds, 1)
		assert.Equal(t, 2, builds[0].ID)
	})

	t.Run("cap drops the oldest", func(t *testing.T) {
		path := buildsFile(t)
		for i := range buildsMax + 5 {
			require.NoError(t, rememberBuildAt(path, t0.Add(time.Duration(i)*time.Second), i+1, "own"))
		}

		builds := readBuilds(path)
		require.Len(t, builds, buildsMax)
		assert.Equal(t, 6, builds[0].ID)
		assert.Equal(t, buildsMax+5, builds[len(builds)-1].ID)
	})
}

func TestBuildsFileMissingOrCorrupted(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		path := buildsFile(t)
		_, ok := observeBuildAt(path, t0, 42, "finished")
		assert.False(t, ok)
		assert.NoFileExists(t, path)
	})

	t.Run("corrupted counts as empty", func(t *testing.T) {
		path := buildsFile(t)
		require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

		_, ok := observeBuildAt(path, t0, 42, "finished")
		assert.False(t, ok)
		require.NoError(t, rememberBuildAt(path, t0, 42, "own"))
		assert.Len(t, readBuilds(path), 1)
	})
}

func TestRememberAndObserveBuild(t *testing.T) {
	setup := func(t *testing.T) string {
		t.Helper()
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		enableAnalytics(t)
		path, err := buildsPath()
		require.NoError(t, err)
		return path
	}

	t.Run("enabled", func(t *testing.T) {
		path := setup(t)
		RememberBuild(42, api.OnboardingRepositoryDemo)
		require.FileExists(t, path)

		ev, ok := ObserveBuild(42, "queued")
		assert.False(t, ok, "a queued build isn't read")
		assert.Empty(t, ev)
		ev, ok = ObserveBuild(42, "finished")
		assert.True(t, ok)
		assert.Equal(t, BuildEvents{Repository: "demo", Started: true, Finished: true}, ev)
	})

	t.Run("nothing written when opted out", func(t *testing.T) {
		path := setup(t)
		t.Setenv(analytics.EnvDoNotTrack, "1")
		RememberBuild(42, api.OnboardingRepositoryOwn)
		assert.NoFileExists(t, path)
	})

	t.Run("opted out after queueing", func(t *testing.T) {
		path := setup(t)
		RememberBuild(42, api.OnboardingRepositoryOwn)
		t.Setenv(analytics.EnvAnalytics, "0")
		_, ok := ObserveBuild(42, "finished")
		assert.False(t, ok)
		assert.Len(t, readBuilds(path), 1, "file left as it was")
	})
}

func TestRepositoryValuesMatchAnalytics(t *testing.T) {
	assert.Equal(t, analytics.OnboardingRepositoryOwn, string(api.OnboardingRepositoryOwn))
	assert.Equal(t, analytics.OnboardingRepositoryDemo, string(api.OnboardingRepositoryDemo))
}
