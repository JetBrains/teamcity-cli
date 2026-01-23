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
	Fields  []string
}

// GetBuildTypes returns a list of build configurations
func (c *Client) GetBuildTypes(opts BuildTypesOptions) (*BuildTypeList, error) {
	locator := NewLocator().
		Add("project", opts.Project).
		AddIntDefault("count", opts.Limit, 30)

	fields := opts.Fields
	if len(fields) == 0 {
		fields = BuildTypeFields.Default
	}
	fieldsParam := fmt.Sprintf("count,buildType(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/buildTypes?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))

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

// SetBuildTypePaused sets the paused state of a build configuration
func (c *Client) SetBuildTypePaused(id string, paused bool) error {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/paused", id)

	resp, err := c.doRequestFull("PUT", path, strings.NewReader(fmt.Sprintf("%t", paused)), "text/plain", "text/plain")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return c.handleErrorResponse(resp)
	}

	return nil
}
