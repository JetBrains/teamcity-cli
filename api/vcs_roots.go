package api

import (
	"fmt"
	"net/url"
)

// GetVcsRoots returns a list of VCS roots
func (c *Client) GetVcsRoots(opts VcsRootsOptions) (*VcsRootList, error) {
	locator := NewLocator().
		Add("affectedProject", opts.Project).
		AddIntDefault("count", opts.Limit, 100)

	fields := opts.Fields
	if len(fields) == 0 {
		fields = VcsRootFields.Default
	}
	fieldsParam := fmt.Sprintf("count,vcs-root(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/vcs-roots?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))

	var result VcsRootList
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetVcsRoot returns a VCS root by ID
func (c *Client) GetVcsRoot(id string) (*VcsRoot, error) {
	path := fmt.Sprintf("/app/rest/vcs-roots/id:%s", id)

	var result VcsRoot
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteVcsRoot deletes a VCS root by ID
func (c *Client) DeleteVcsRoot(id string) error {
	path := fmt.Sprintf("/app/rest/vcs-roots/id:%s", id)
	return c.doNoContent("DELETE", path, nil, "")
}
