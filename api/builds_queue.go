package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
)

type buildCancelRequest struct {
	Comment        string `json:"comment"`
	ReaddIntoQueue bool   `json:"readdIntoQueue"`
}

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

// RemoveFromQueue cancels a queued build; DELETE would also erase its history entry, which needs far stronger permissions than canceling.
func (c *Client) RemoveFromQueue(id string) error {
	return c.cancelQueued(id, "")
}

func (c *Client) cancelQueued(id, comment string) error {
	body, err := json.Marshal(buildCancelRequest{Comment: comment})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	return c.doNoContent(c.ctx(), "POST", "/app/rest/buildQueue/id:"+id, bytes.NewReader(body), "application/json")
}

// MoveQueuedBuildToTop moves a queued build to the top of the queue; the path takes the position ("first", "last" or "after:<ids>"), the body the build.
func (c *Client) MoveQueuedBuildToTop(buildID string) error {
	body, err := json.Marshal(map[string]string{"id": buildID})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	return c.doNoContent(c.ctx(), "PUT", "/app/rest/buildQueue/order/first", bytes.NewReader(body), "application/json")
}

// ApproveQueuedBuild approves a queued build that requires approval
func (c *Client) ApproveQueuedBuild(buildID string) error {
	path := fmt.Sprintf("/app/rest/buildQueue/id:%s/approve", buildID)
	return c.doNoContent(c.ctx(), "POST", path, nil, "")
}

// GetQueuedBuildApprovalInfo returns approval information for a queued build
func (c *Client) GetQueuedBuildApprovalInfo(buildID string) (*ApprovalInfo, error) {
	path := fmt.Sprintf("/app/rest/buildQueue/id:%s/approvalInfo", buildID)

	var info ApprovalInfo
	if err := c.get(c.ctx(), path, &info); err != nil {
		return nil, err
	}

	return &info, nil
}
