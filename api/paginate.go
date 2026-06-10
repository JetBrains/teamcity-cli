package api

import (
	"net/url"
	"strings"
)

// allPageSize is the per-page count requested for an unbounded fetch; the server caps it and collectPages follows nextHref for the remainder, so a large value just trims round-trips.
const allPageSize = 1000

// pageCount returns the per-page item count to request: allPageSize when limit is unbounded (0), otherwise the limit.
func pageCount(limit int) int {
	if limit == 0 {
		return allPageSize
	}
	return limit
}

// collectPages follows NextHref links to accumulate items up to the limit (0 collects all); the bool is true when a finite limit capped the result and more exist.
func collectPages[T any](c *Client, path string, limit int, fetch func(string) ([]T, string, error)) ([]T, bool, error) {
	all := []T{} // non-nil so an empty result serializes as JSON [] not null
	for path != "" {
		items, nextHref, err := fetch(path)
		if err != nil {
			return nil, false, err
		}
		all = append(all, items...)
		if limit > 0 && len(all) >= limit {
			truncated := len(all) > limit || c.normalizePaginationPath(nextHref) != ""
			return all[:limit], truncated, nil
		}
		path = c.normalizePaginationPath(nextHref)
	}
	return all, false, nil
}

// normalizePaginationPath converts a TeamCity NextHref value into a path
// suitable for c.get. It strips the scheme/host, context path, guestAuth
// prefix, and API version so that apiPath() can re-apply them consistently.
func (c *Client) normalizePaginationPath(href string) string {
	if href == "" {
		return ""
	}

	path := href

	// Absolute URL → path+query only
	if u, err := url.Parse(path); err == nil && u.IsAbs() {
		path = u.RequestURI()
	}

	// Strip context path (e.g. /teamcity)
	if base, err := url.Parse(c.BaseURL); err == nil {
		basePath := strings.TrimSuffix(base.Path, "/")
		if len(basePath) > 1 {
			path = strings.TrimPrefix(path, basePath)
		}
	}

	// Strip guestAuth prefix so the version check below works,
	// then let apiPath() re-add it if needed.
	path = strings.TrimPrefix(path, "/guestAuth")

	// Strip API version prefix; apiPath() will re-add it.
	if c.APIVersion != "" {
		versionedPrefix := "/app/rest/" + c.APIVersion + "/"
		if after, ok := strings.CutPrefix(path, versionedPrefix); ok {
			path = "/app/rest/" + after
		}
	}

	return path
}
