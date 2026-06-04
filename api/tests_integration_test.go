//go:build integration || guest

package api_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTests_Integration(T *testing.T) {
	T.Parallel()

	if testProject == "" {
		T.Skip("no test project available")
	}

	T.Run("failing by project", func(t *testing.T) {
		t.Parallel()

		occ, err := client.ListTests(t.Context(), api.TestQueryOptions{Project: testProject, Failing: true, Limit: 10})
		require.NoError(t, err)
		require.NotNil(t, occ)
		t.Logf("currently failing in %s: count=%d", testProject, occ.Count)
		for _, o := range occ.TestOccurrence {
			assert.NotEmpty(t, o.Name, "occurrence should have a name")
		}
	})

	T.Run("muted by project", func(t *testing.T) {
		t.Parallel()

		occ, err := client.ListTests(t.Context(), api.TestQueryOptions{Project: testProject, Muted: true, Limit: 10})
		require.NoError(t, err)
		require.NotNil(t, occ)
		t.Logf("currently muted in %s: count=%d", testProject, occ.Count)
	})

	T.Run("by job", func(t *testing.T) {
		t.Parallel()

		if testConfig == "" {
			t.Skip("no test config available")
		}
		occ, err := client.ListTests(t.Context(), api.TestQueryOptions{Job: testConfig, Failing: true, Limit: 10})
		require.NoError(t, err)
		require.NotNil(t, occ)
		t.Logf("currently failing in %s: count=%d", testConfig, occ.Count)
	})

	T.Run("scope required", func(t *testing.T) {
		t.Parallel()

		_, err := client.ListTests(t.Context(), api.TestQueryOptions{Failing: true})
		assert.Error(t, err, "unscoped query must be rejected")
	})
}

func TestGetTestHistory_Integration(T *testing.T) {
	T.Parallel()

	if testProject == "" {
		T.Skip("no test project available")
	}

	// Discover a real test name from the build's occurrences; fall back to skipping.
	name := discoverTestName(T)
	if name == "" {
		T.Skip("no test occurrences available to build a history query")
	}

	occ, err := client.GetTestHistory(T.Context(), name, api.TestQueryOptions{Project: testProject, Limit: 25})
	require.NoError(T, err)
	require.NotNil(T, occ)
	T.Logf("history of %q in %s: count=%d", name, testProject, occ.Count)
	for _, o := range occ.TestOccurrence {
		assert.NotEmpty(T, o.Status, "history entry should have a status")
		assert.NotNil(T, o.Build, "history entry should embed its build")
	}

	T.Run("scope required", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetTestHistory(t.Context(), name, api.TestQueryOptions{})
		assert.Error(t, err, "unscoped history must be rejected")
	})
}

func TestResolveTestID_Integration(T *testing.T) {
	T.Parallel()

	if testProject == "" {
		T.Skip("no test project available")
	}

	name := discoverTestName(T)
	if name == "" {
		T.Skip("no test occurrences available to resolve")
	}

	id, err := client.ResolveTestID(T.Context(), name, testProject)
	if err != nil {
		var ambiguous *api.AmbiguousTestError
		if errors.As(err, &ambiguous) {
			require.Greater(T, len(ambiguous.Candidates), 1, "ambiguous error should carry candidates")
			T.Logf("name %q is ambiguous across %d tests", name, len(ambiguous.Candidates))
			return
		}
		T.Fatalf("ResolveTestID: %v", err)
	}
	assert.NotEmpty(T, id, "resolved id should not be empty")
	T.Logf("resolved %q -> %s", name, id)

	T.Run("not found", func(t *testing.T) {
		t.Parallel()

		_, err := client.ResolveTestID(t.Context(), "tc.cli.definitely.missing.test.0xDEADBEEF", testProject)
		assert.Error(t, err, "unknown name should error")
	})
}

func TestMuteRoundTrip_Integration(T *testing.T) {
	skipIfGuest(T)

	if testProject == "" {
		T.Skip("no test project available")
	}

	testID := resolveAnyTestID(T)
	if testID == "" {
		T.Skip("no resolvable test id available")
	}

	scope := api.ProblemScopeOptions{Project: testProject}

	// Clean up any pre-existing mutes for this test so the round-trip starts fresh.
	cleanupMutes(T, testID, scope)
	T.Cleanup(func() { cleanupMutes(T, testID, scope) })

	mute, err := client.CreateMute(T.Context(), testID, scope, api.MuteOptions{Reason: "integration test mute"})
	require.NoError(T, err)
	require.NotNil(T, mute)
	T.Logf("created mute id=%d", mute.ID)

	mutes, err := client.ListMutes(T.Context(), testID, scope)
	require.NoError(T, err)
	require.NotNil(T, mutes)
	assert.Greater(T, mutes.Count, 0, "the created mute should be listed")

	require.Greater(T, len(mutes.Mute), 0)
	err = client.DeleteMute(T.Context(), mutes.Mute[0].ID)
	require.NoError(T, err)

	after, err := client.ListMutes(T.Context(), testID, scope)
	require.NoError(T, err)
	assert.Equal(T, 0, after.Count, "mute should be gone after delete")
}

func TestInvestigationRoundTrip_Integration(T *testing.T) {
	skipIfGuest(T)

	if testProject == "" {
		T.Skip("no test project available")
	}

	testID := resolveAnyTestID(T)
	if testID == "" {
		T.Skip("no resolvable test id available")
	}

	scope := api.ProblemScopeOptions{Project: testProject}

	// Best-effort resolve any leftover investigation, then ensure cleanup.
	_ = client.ResolveInvestigation(T.Context(), testID, scope, "FIXED")
	T.Cleanup(func() {
		_ = client.ResolveInvestigation(T.Context(), testID, scope, "FIXED")
	})

	inv, err := client.CreateInvestigation(T.Context(), testID, scope, "")
	require.NoError(T, err)
	require.NotNil(T, inv)
	assert.Equal(T, "TAKEN", inv.State)
	T.Logf("created investigation id=%s state=%s", inv.ID, inv.State)

	err = client.ResolveInvestigation(T.Context(), testID, scope, "FIXED")
	require.NoError(T, err)
}

// discoverTestName returns a test name from the seed build's occurrences, or "".
func discoverTestName(t *testing.T) string {
	t.Helper()
	if testBuild == nil {
		return ""
	}
	tests, err := client.GetBuildTests(t.Context(), fmt.Sprintf("%d", testBuild.ID), api.BuildTestsOptions{Limit: 1})
	if err != nil || tests.Count == 0 || len(tests.TestOccurrence) == 0 {
		return ""
	}
	return tests.TestOccurrence[0].Name
}

// resolveAnyTestID resolves a seed test name to a test id, returning "" when unavailable or ambiguous.
func resolveAnyTestID(t *testing.T) string {
	t.Helper()
	name := discoverTestName(t)
	if name == "" {
		return ""
	}
	id, err := client.ResolveTestID(t.Context(), name, testProject)
	if err != nil {
		t.Logf("resolveAnyTestID(%q): %v", name, err)
		return ""
	}
	return id
}

// cleanupMutes deletes every mute currently targeting testID in scope.
func cleanupMutes(t *testing.T, testID string, scope api.ProblemScopeOptions) {
	t.Helper()
	mutes, err := client.ListMutes(t.Context(), testID, scope)
	if err != nil {
		t.Logf("cleanupMutes list: %v", err)
		return
	}
	for _, m := range mutes.Mute {
		if err := client.DeleteMute(t.Context(), m.ID); err != nil {
			t.Logf("cleanupMutes delete id=%d: %v", m.ID, err)
		}
	}
}
