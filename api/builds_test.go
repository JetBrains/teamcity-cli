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

func TestGetBuildsUsesFavoritesLocator(T *testing.T) {
	T.Parallel()

	var capturedQuery string
	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildList{Count: 0, Builds: []Build{}})
	})

	_, err := client.GetBuilds(T.Context(), BuildsOptions{Favorites: true, Limit: 5})
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
