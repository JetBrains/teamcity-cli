package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// GetVcsRoots returns a list of VCS roots, automatically following pagination.
func (c *Client) GetVcsRoots(opts VcsRootsOptions) (*VcsRootList, error) {
	locator := NewLocator().
		Add("affectedProject", opts.Project).
		AddIntDefault("count", opts.Limit, 100)

	fields := opts.Fields
	if len(fields) == 0 {
		fields = VcsRootFields.Default
	}
	fieldsParam := fmt.Sprintf("count,nextHref,vcs-root(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/vcs-roots?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))

	roots, err := collectPages(c, path, opts.Limit, func(p string) ([]VcsRoot, string, error) {
		var page VcsRootList
		if err := c.get(context.Background(), p, &page); err != nil {
			return nil, "", err
		}
		return page.VcsRoot, page.NextHref, nil
	})
	if err != nil {
		return nil, err
	}
	return &VcsRootList{Count: len(roots), VcsRoot: roots}, nil
}

// GetVcsRoot returns a VCS root by ID
func (c *Client) GetVcsRoot(id string) (*VcsRoot, error) {
	path := "/app/rest/vcs-roots/id:" + id

	var result VcsRoot
	if err := c.get(context.Background(), path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteVcsRoot deletes a VCS root by ID
func (c *Client) DeleteVcsRoot(id string) error {
	path := "/app/rest/vcs-roots/id:" + id
	return c.doNoContent(context.Background(), "DELETE", path, nil, "")
}

// CreateVcsRoot creates a new VCS root
func (c *Client) CreateVcsRoot(root VcsRoot) (*VcsRoot, error) {
	body, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal VCS root: %w", err)
	}

	var result VcsRoot
	if err := c.post(context.Background(), "/app/rest/vcs-roots", bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TestVcsConnection tests a VCS connection before creating a root.
// Uses the pipeline endpoint which returns HTTP 200 with status/errors in the body.
func (c *Client) TestVcsConnection(req TestConnectionRequest, projectID string) (*TestConnectionResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	path := "/app/pipeline/repository/testConnection?parentProjectExtId=" + projectID
	resp, err := c.doRequest(context.Background(), "POST", path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := ExtractErrorMessage(respBody)
		if msg == "" {
			msg = fmt.Sprintf("connection test failed (status %d)", resp.StatusCode)
		}
		return &TestConnectionResult{
			Status: "ERROR",
			Errors: []TestConnectionError{{Message: msg}},
		}, nil
	}

	var result TestConnectionResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse test connection response: %w", err)
	}
	return &result, nil
}
