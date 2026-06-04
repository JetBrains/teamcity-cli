package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// QueueOptions represents options for listing queued builds
type QueueOptions struct {
	BuildTypeID string
	Limit       int
	Fields      []string
}

// GetBuildQueue returns the build queue, following pagination; the bool is true when a finite limit capped the result.
func (c *Client) GetBuildQueue(opts QueueOptions) (*BuildQueue, bool, error) {
	locator := NewLocator().
		Add("buildType", opts.BuildTypeID).
		AddInt("count", pageCount(opts.Limit))

	fields := opts.Fields
	if len(fields) == 0 {
		fields = QueuedBuildFields.Default
	}
	fieldsParam := fmt.Sprintf("count,nextHref,build(%s)", ToAPIFields(fields))

	path := "/app/rest/buildQueue"
	if !locator.IsEmpty() {
		path = fmt.Sprintf("%s?locator=%s&fields=%s", path, locator.Encode(), url.QueryEscape(fieldsParam))
	} else {
		path = fmt.Sprintf("%s?fields=%s", path, url.QueryEscape(fieldsParam))
	}

	builds, truncated, err := collectPages(c, path, opts.Limit, func(p string) ([]QueuedBuild, string, error) {
		var page BuildQueue
		if err := c.get(c.ctx(), p, &page); err != nil {
			return nil, "", err
		}
		return page.Builds, page.NextHref, nil
	})
	if err != nil {
		return nil, false, err
	}
	return &BuildQueue{Count: len(builds), Builds: builds}, truncated, nil
}

// RemoveFromQueue removes a build from the queue
func (c *Client) RemoveFromQueue(id string) error {
	path := "/app/rest/buildQueue/id:" + id
	return c.doNoContent(c.ctx(), "DELETE", path, nil, "")
}

// SetQueuedBuildPosition moves a queued build to a specific position in the queue
func (c *Client) SetQueuedBuildPosition(buildID string, position int) error {
	path := "/app/rest/buildQueue/order/" + buildID
	return c.doNoContent(c.ctx(), "PUT", path, strings.NewReader(strconv.Itoa(position)), "text/plain")
}

// MoveQueuedBuildToTop moves a queued build to the top of the queue
func (c *Client) MoveQueuedBuildToTop(buildID string) error {
	return c.SetQueuedBuildPosition(buildID, 0)
}

// ApproveQueuedBuild approves a queued build that requires approval
func (c *Client) ApproveQueuedBuild(buildID string) error {
	path := fmt.Sprintf("/app/rest/buildQueue/id:%s/approval/status", buildID)
	return c.doNoContent(c.ctx(), "PUT", path, strings.NewReader(`"approved"`), "application/json")
}

// GetQueuedBuildApprovalInfo returns approval information for a queued build
func (c *Client) GetQueuedBuildApprovalInfo(buildID string) (*ApprovalInfo, error) {
	path := fmt.Sprintf("/app/rest/buildQueue/id:%s/approval", buildID)

	var info ApprovalInfo
	if err := c.get(c.ctx(), path, &info); err != nil {
		return nil, err
	}

	return &info, nil
}
