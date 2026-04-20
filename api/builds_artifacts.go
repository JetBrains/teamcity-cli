package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// encodeArtifactPath escapes each segment of an artifact path individually,
// preserving "/" as path separators.
func encodeArtifactPath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

// Artifact represents a build artifact
type Artifact struct {
	Name     string     `json:"name"`
	Size     int64      `json:"size,omitempty"`
	ModTime  string     `json:"modificationTime,omitempty"`
	Href     string     `json:"href,omitempty"`
	Children *Artifacts `json:"children,omitempty"`
	Content  *Content   `json:"content,omitempty"`
}

// Content represents artifact content reference
type Content struct {
	Href string `json:"href"`
}

// Artifacts represents a list of artifacts
type Artifacts struct {
	Count int        `json:"count"`
	File  []Artifact `json:"file"`
}

// GetArtifacts returns the artifacts for a build (accepts ID or #number).
// If subpath is non-empty, it lists artifacts under that subdirectory.
func (c *Client) GetArtifacts(ctx context.Context, buildID string, subpath string) (*Artifacts, error) {
	id, err := c.ResolveBuildID(ctx, buildID)
	if err != nil {
		return nil, err
	}
	p := fmt.Sprintf("/app/rest/builds/id:%s/artifacts/children", id)
	if subpath != "" {
		p += "/" + encodeArtifactPath(subpath)
	}

	var artifacts Artifacts
	if err := c.get(ctx, p, &artifacts); err != nil {
		return nil, err
	}

	return &artifacts, nil
}

// DownloadArtifact downloads an artifact and returns its content (accepts ID or #number)
func (c *Client) DownloadArtifact(ctx context.Context, buildID, artifactPath string) ([]byte, error) {
	id, err := c.ResolveBuildID(ctx, buildID)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/artifacts/content/%s", id, encodeArtifactPath(artifactPath))

	resp, err := c.doGetStream(ctx, path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	return io.ReadAll(resp.Body)
}

// DownloadArtifactTo streams an artifact to w using a timeout-less client bounded by ctx.
func (c *Client) DownloadArtifactTo(ctx context.Context, buildID, artifactPath string, w io.Writer) (int64, error) {
	id, err := c.ResolveBuildID(ctx, buildID)
	if err != nil {
		return 0, err
	}

	path := fmt.Sprintf("/app/rest/builds/id:%s/artifacts/content/%s", id, encodeArtifactPath(artifactPath))
	reqURL := fmt.Sprintf("%s%s", c.BaseURL, c.apiPath(path))
	streamClient := &http.Client{Transport: c.HTTPClient.Transport}

	resp, err := withRetry(ReadRetry, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		c.setAuth(req)
		return streamClient.Do(req)
	})
	if err != nil {
		if resp != nil {
			defer func() { _ = resp.Body.Close() }()
			return 0, c.handleErrorResponse(resp)
		}
		return 0, &NetworkError{URL: c.BaseURL, Cause: err}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, c.handleErrorResponse(resp)
	}

	return io.Copy(w, resp.Body)
}

// GetBuildLog returns the build log (accepts ID or #number)
func (c *Client) GetBuildLog(ctx context.Context, buildID string) (string, error) {
	id, err := c.ResolveBuildID(ctx, buildID)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/downloadBuildLog.html?buildId=%s", id)

	resp, err := c.doGetStream(ctx, path)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
