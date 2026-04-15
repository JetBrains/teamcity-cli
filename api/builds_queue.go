package api

import (
	"fmt"
	"net/url"
	"strings"
)

// QueueOptions represents options for listing queued builds
type QueueOptions struct {
	BuildTypeID  string
	Limit        int
	Skip         int
	ContinuePath string
	Fields       []string
}

// GetBuildQueue returns the build queue
func (c *Client) GetBuildQueue(opts QueueOptions) (*BuildQueue, error) {
	fields := opts.Fields
	if len(fields) == 0 {
		fields = QueuedBuildFields.Default
	}
	fieldsParam := paginatedFieldsParam("build", fields)

	path := opts.ContinuePath
	if path != "" {
		var err error
		path, err = rewriteContinuationPath(path, opts.Limit, fieldsParam)
		if err != nil {
			return nil, err
		}
	} else {
		locator := NewLocator().
			Add("buildType", opts.BuildTypeID).
			AddInt("count", opts.Limit).
			AddInt("start", opts.Skip)

		path = "/app/rest/buildQueue"
		if !locator.IsEmpty() {
			path = fmt.Sprintf("%s?locator=%s&fields=%s", path, locator.Encode(), url.QueryEscape(fieldsParam))
		} else {
			path = fmt.Sprintf("%s?fields=%s", path, url.QueryEscape(fieldsParam))
		}
	}

	var queue BuildQueue
	if err := c.get(path, &queue); err != nil {
		return nil, err
	}
	return &queue, nil
}

// RemoveFromQueue removes a build from the queue
func (c *Client) RemoveFromQueue(id string) error {
	path := fmt.Sprintf("/app/rest/buildQueue/id:%s", id)
	return c.doNoContent("DELETE", path, nil, "")
}

// SetQueuedBuildPosition moves a queued build to a specific position in the queue
func (c *Client) SetQueuedBuildPosition(buildID string, position int) error {
	path := fmt.Sprintf("/app/rest/buildQueue/order/%s", buildID)
	return c.doNoContent("PUT", path, strings.NewReader(fmt.Sprintf("%d", position)), "text/plain")
}

// MoveQueuedBuildToTop moves a queued build to the top of the queue
func (c *Client) MoveQueuedBuildToTop(buildID string) error {
	return c.SetQueuedBuildPosition(buildID, 0)
}

// ApproveQueuedBuild approves a queued build that requires approval
func (c *Client) ApproveQueuedBuild(buildID string) error {
	path := fmt.Sprintf("/app/rest/buildQueue/id:%s/approval/status", buildID)
	return c.doNoContent("PUT", path, strings.NewReader(`"approved"`), "application/json")
}

// GetQueuedBuildApprovalInfo returns approval information for a queued build
func (c *Client) GetQueuedBuildApprovalInfo(buildID string) (*ApprovalInfo, error) {
	path := fmt.Sprintf("/app/rest/buildQueue/id:%s/approval", buildID)

	var info ApprovalInfo
	if err := c.get(path, &info); err != nil {
		return nil, err
	}

	return &info, nil
}
