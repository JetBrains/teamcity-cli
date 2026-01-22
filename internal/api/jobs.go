package api

import (
	"fmt"
	"strings"
)

// BuildTypesOptions represents options for listing build configurations
type BuildTypesOptions struct {
	Project string
	Limit   int
}

// GetBuildTypes returns a list of build configurations
func (c *Client) GetBuildTypes(opts BuildTypesOptions) (*BuildTypeList, error) {
	locator := NewLocator().
		Add("project", opts.Project).
		AddIntDefault("count", opts.Limit, 30)

	path := fmt.Sprintf("/app/rest/buildTypes?locator=%s", locator.Encode())

	var result BuildTypeList
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetBuildType returns a single build configuration by ID
func (c *Client) GetBuildType(id string) (*BuildType, error) {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s", id)

	var buildType BuildType
	if err := c.get(path, &buildType); err != nil {
		return nil, err
	}

	return &buildType, nil
}

// PauseBuildType pauses a build configuration
func (c *Client) PauseBuildType(id string) error {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/paused", id)

	resp, err := c.doRequestFull("PUT", path, strings.NewReader("true"), "text/plain", "text/plain")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// ResumeBuildType resumes a paused build configuration
func (c *Client) ResumeBuildType(id string) error {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/paused", id)

	resp, err := c.doRequestFull("PUT", path, strings.NewReader("false"), "text/plain", "text/plain")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return c.handleErrorResponse(resp)
	}

	return nil
}
