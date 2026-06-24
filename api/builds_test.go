package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildsOptionsLocator(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name   string
		opts   BuildsOptions
		want   []string
		reject []string
	}{
		{
			name: "defaults include all branches and disable server default filters",
			opts: BuildsOptions{},
			want: []string{
				"defaultFilter:false",
				"branch:(default:any)",
				"lookupLimit:5000",
			},
		},
		{
			name: "revision filter adds revision dimension",
			opts: BuildsOptions{
				Revision: "abc1234def5678",
			},
			want: []string{
				"revision:abc1234def5678",
			},
		},
		{
			name: "branch with locator metacharacters goes through the base64 name condition",
			opts: BuildsOptions{
				Branch: "release(2024)",
			},
			want: []string{
				"branch:(name:(value:($base64:cmVsZWFzZSgyMDI0KQ)))",
			},
			reject: []string{
				"branch:(release",
				"branch:(default:any)",
			},
		},
		{
			name: "favorites use current user star tag locator",
			opts: BuildsOptions{
				BuildTypeID: "MyBuild",
				Branch:      "main",
				Status:      "success",
				User:        "alice",
				Favorites:   true,
			},
			want: []string{
				"buildType:MyBuild",
				"branch:main",
				"status:SUCCESS",
				"user:alice",
				"tag:(private:true,owner:current,condition:(value:.teamcity.star,matchType:equals,ignoreCase:false))",
			},
			reject: []string{
				"branch:(default:any)",
				"lookupLimit",
			},
		},
		{
			name: "deep lookup (exact number) skips the unscoped lookup-limit cap",
			opts: BuildsOptions{Number: "123", DeepLookup: true},
			want: []string{
				"number:123",
			},
			reject: []string{
				"lookupLimit",
			},
		},
	}

	for _, tt := range tests {
		T.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.opts.Locator().String()
			for _, want := range tt.want {
				assert.Contains(t, got, want)
			}
			for _, reject := range tt.reject {
				assert.NotContains(t, got, reject)
			}
		})
	}
}

func TestGetBuildsStopsOnEmptyPageUnlessDeepLookup(T *testing.T) {
	newServer := func() (*Client, *int) {
		calls := new(int)
		c := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
			*calls++
			w.Header().Set("Content-Type", "application/json")
			if *calls == 1 {
				// Empty window with a nextHref — TeamCity's lookupLimit-escalation signal.
				_ = json.NewEncoder(w).Encode(BuildList{Count: 0, NextHref: "/app/rest/builds?cursor=2", Builds: []Build{}})
				return
			}
			_ = json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 42}}})
		})
		return c, calls
	}

	T.Run("list query stops at the first empty page", func(t *testing.T) {
		c, calls := newServer()
		builds, _, err := c.GetBuilds(t.Context(), BuildsOptions{Number: "123", Limit: 1})
		require.NoError(t, err)
		assert.Equal(t, 1, *calls, "must not chase the lookupLimit escalation")
		assert.Equal(t, 0, builds.Count)
	})

	T.Run("deep lookup follows nextHref to find an old build", func(t *testing.T) {
		c, calls := newServer()
		builds, _, err := c.GetBuilds(t.Context(), BuildsOptions{Number: "123", Limit: 1, DeepLookup: true})
		require.NoError(t, err)
		assert.Equal(t, 2, *calls, "must follow nextHref past the empty page")
		require.Equal(t, 1, builds.Count)
		assert.Equal(t, 42, builds.Builds[0].ID)
	})
}

func TestUnscopedLookupLimitEnvOverride(T *testing.T) {
	T.Setenv(envLookupLimit, "250")
	assert.Equal(T, 250, unscopedLookupLimit(), "positive override applies")
	T.Setenv(envLookupLimit, "nope")
	assert.Equal(T, 5000, unscopedLookupLimit(), "invalid value falls back to default")
	T.Setenv(envLookupLimit, "0")
	assert.Equal(T, 5000, unscopedLookupLimit(), "non-positive falls back to default")
}

func TestGetBuildsUsesFavoritesLocator(T *testing.T) {
	T.Parallel()

	var capturedQuery string
	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildList{Count: 0, Builds: []Build{}})
	})

	_, _, err := client.GetBuilds(T.Context(), BuildsOptions{Favorites: true, Limit: 5})
	require.NoError(T, err)

	assert.Contains(T, capturedQuery, BuildsOptions{Favorites: true}.Locator().Encode())
	assert.Contains(T, capturedQuery, "count%3A5")
}

func TestRunBuildSendsSnapshotDependencies(T *testing.T) {
	T.Parallel()

	var captured TriggerBuildRequest
	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(T, err)
		require.NoError(T, json.Unmarshal(body, &captured))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Build{ID: 1})
	})

	_, err := client.RunBuild("MyBuild", RunBuildOptions{
		SnapshotDependencies: []int{6946, 6917, 6922},
	})
	require.NoError(T, err)

	require.NotNil(T, captured.SnapshotDependencies)
	require.Len(T, captured.SnapshotDependencies.Build, 3)
	assert.Equal(T, 6946, captured.SnapshotDependencies.Build[0].ID)
	assert.Equal(T, 6917, captured.SnapshotDependencies.Build[1].ID)
	assert.Equal(T, 6922, captured.SnapshotDependencies.Build[2].ID)
}

func TestRunBuildOmitsEmptySnapshotDependencies(T *testing.T) {
	T.Parallel()

	var rawBody []byte
	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(T, err)
		rawBody = b
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Build{ID: 1})
	})

	_, err := client.RunBuild("MyBuild", RunBuildOptions{})
	require.NoError(T, err)
	assert.NotContains(T, string(rawBody), "snapshot-dependencies")
}
