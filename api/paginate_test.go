package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectPages(t *testing.T) {
	t.Parallel()

	t.Run("multiple pages", func(t *testing.T) {
		t.Parallel()
		c := &Client{BaseURL: "http://localhost"}
		call := 0
		items, err := collectPages(c, "/app/rest/builds?page=1", 0, func(path string) ([]int, string, error) {
			call++
			switch call {
			case 1:
				return []int{1, 2}, "/app/rest/builds?page=2", nil
			case 2:
				return []int{3, 4}, "/app/rest/builds?page=3", nil
			default:
				return []int{5}, "", nil
			}
		})
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3, 4, 5}, items)
		assert.Equal(t, 3, call)
	})

	t.Run("stops at limit", func(t *testing.T) {
		t.Parallel()
		c := &Client{BaseURL: "http://localhost"}
		call := 0
		items, err := collectPages(c, "/app/rest/builds", 3, func(path string) ([]int, string, error) {
			call++
			return []int{call * 10, call*10 + 1}, "/app/rest/builds?next", nil
		})
		require.NoError(t, err)
		assert.Equal(t, []int{10, 11, 20}, items)
		assert.Equal(t, 2, call)
	})
}

func TestNormalizePaginationPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		baseURL    string
		apiVersion string
		href       string
		want       string
	}{
		{
			name:    "absolute URL stripped to path",
			baseURL: "http://localhost",
			href:    "http://localhost/app/rest/builds?locator=count:30",
			want:    "/app/rest/builds?locator=count:30",
		},
		{
			name:    "context path stripped",
			baseURL: "http://localhost/teamcity",
			href:    "/teamcity/app/rest/builds?locator=count:30",
			want:    "/app/rest/builds?locator=count:30",
		},
		{
			name:       "guestAuth and version stripped",
			baseURL:    "http://localhost",
			apiVersion: "2020.1",
			href:       "/guestAuth/app/rest/2020.1/builds?locator=count:30",
			want:       "/app/rest/builds?locator=count:30",
		},
		{
			name:       "context path and guestAuth and version",
			baseURL:    "http://localhost/teamcity",
			apiVersion: "2020.1",
			href:       "http://localhost/teamcity/guestAuth/app/rest/2020.1/builds?locator=count:30",
			want:       "/app/rest/builds?locator=count:30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Client{BaseURL: tt.baseURL, APIVersion: tt.apiVersion}
			got := c.normalizePaginationPath(tt.href)
			assert.Equal(t, tt.want, got)
		})
	}
}
