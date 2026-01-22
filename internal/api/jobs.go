package api

import (
	"fmt"
	"net/url"
	"strings"
)

// BuildTypesOptions represents options for listing build configurations
type BuildTypesOptions struct {
	Project string
	Limit   int
}

// GetBuildTypes returns a list of build configurations
func (c *Client) GetBuildTypes(opts BuildTypesOptions) (*BuildTypeList, error) {
	var locatorParts []string

	if opts.Project != "" {
		locatorParts = append(locatorParts, fmt.Sprintf("project:%s", opts.Project))
	}
	if opts.Limit > 0 {
		locatorParts = append(locatorParts, fmt.Sprintf("count:%d", opts.Limit))
	} else {
		locatorParts = append(locatorParts, "count:30")
	}

	path := "/app/rest/buildTypes"
	if len(locatorParts) > 0 {
		locator := strings.Join(locatorParts, ",")
		path = fmt.Sprintf("%s?locator=%s", path, url.QueryEscape(locator))
	}

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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return c.handleErrorResponse(resp)
	}

	return nil
}
