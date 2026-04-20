package api

import (
	"net/url"
	"strings"
)

// collectPages follows NextHref links to accumulate items up to the limit.
// If limit is 0, all pages are collected.
func collectPages[T any](c *Client, path string, limit int, fetch func(string) ([]T, string, error)) ([]T, error) {
	var all []T
	for path != "" {
		items, nextHref, err := fetch(path)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if limit > 0 && len(all) >= limit {
			return all[:limit], nil
		}
		path = c.normalizePaginationPath(nextHref)
	}
	return all, nil
}

// normalizePaginationPath converts a TeamCity NextHref value into a path
// suitable for c.get(context.Background(), ). It strips the scheme/host, context path, guestAuth
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
