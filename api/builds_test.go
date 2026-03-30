package api

import (
	"encoding/json"
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

	_, err := client.GetBuilds(BuildsOptions{Favorites: true, Limit: 5})
	require.NoError(T, err)

	assert.Contains(T, capturedQuery, BuildsOptions{Favorites: true}.Locator().Encode())
	assert.Contains(T, capturedQuery, "count%3A5")
}
