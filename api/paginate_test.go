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
		items, truncated, err := collectPages(c, "/app/rest/builds?page=1", 0, func(path string) ([]int, string, error) {
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
		assert.False(t, truncated)
	})

	t.Run("stops at limit", func(t *testing.T) {
		t.Parallel()
		c := &Client{BaseURL: "http://localhost"}
		call := 0
		items, truncated, err := collectPages(c, "/app/rest/builds", 3, func(path string) ([]int, string, error) {
			call++
			return []int{call * 10, call*10 + 1}, "/app/rest/builds?next", nil
		})
		require.NoError(t, err)
		assert.Equal(t, []int{10, 11, 20}, items)
		assert.Equal(t, 2, call)
		// Collected 4 items for a limit of 3 -> more exist.
		assert.True(t, truncated)
	})

	t.Run("truncated at exact boundary with more pages", func(t *testing.T) {
		t.Parallel()
		c := &Client{BaseURL: "http://localhost"}
		call := 0
		items, truncated, err := collectPages(c, "/app/rest/builds", 4, func(path string) ([]int, string, error) {
			call++
			return []int{call * 10, call*10 + 1}, "/app/rest/builds?next", nil
		})
		require.NoError(t, err)
		assert.Equal(t, []int{10, 11, 20, 21}, items)
		assert.Equal(t, 2, call)
		// Exactly at the limit but nextHref is non-empty -> more exist.
		assert.True(t, truncated)
	})

	t.Run("exhausted exactly at limit reports not truncated", func(t *testing.T) {
		t.Parallel()
		c := &Client{BaseURL: "http://localhost"}
		call := 0
		items, truncated, err := collectPages(c, "/app/rest/builds", 4, func(path string) ([]int, string, error) {
			call++
			next := "/app/rest/builds?next"
			if call == 2 {
				next = ""
			}
			return []int{call * 10, call*10 + 1}, next, nil
		})
		require.NoError(t, err)
		assert.Equal(t, []int{10, 11, 20, 21}, items)
		assert.Equal(t, 2, call)
		// Exactly at the limit and no more pages -> not truncated.
		assert.False(t, truncated)
	})

	t.Run("unbounded fetch uses large page and collects all", func(t *testing.T) {
		t.Parallel()
		c := &Client{BaseURL: "http://localhost"}
		const total = 2500
		pageSize := pageCount(0)
		served, call := 0, 0
		items, truncated, err := collectPages(c, "/app/rest/builds", 0, func(path string) ([]int, string, error) {
			call++
			n := min(pageSize, total-served)
			page := make([]int, n)
			for i := range page {
				page[i] = served + i
			}
			served += n
			next := "/app/rest/builds?next"
			if served >= total {
				next = ""
			}
			return page, next, nil
		})
		require.NoError(t, err)
		assert.Len(t, items, total)
		// 1000 + 1000 + 500 = 3 round-trips, vs 84 at the old page size of 30.
		assert.Equal(t, 3, call)
		// limit == 0 is always unbounded -> never truncated.
		assert.False(t, truncated)
	})
}

func TestPageCount(t *testing.T) {
	t.Parallel()
	assert.Equal(t, allPageSize, pageCount(0))
	assert.Equal(t, 50, pageCount(50))
	assert.Equal(t, 1, pageCount(1))
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
