package api

import (
	"bytes"
	"encoding/json"
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
		Add("affectedProject", opts.Project).
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

// CreateBuildTypeRequest represents a request to create a build configuration
type CreateBuildTypeRequest struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}

// CreateBuildType creates a new build configuration in a project
func (c *Client) CreateBuildType(projectID string, req CreateBuildTypeRequest) (*BuildType, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/app/rest/projects/id:%s/buildTypes", projectID)

	var buildType BuildType
	if err := c.post(path, bytes.NewReader(body), &buildType); err != nil {
		return nil, err
	}

	return &buildType, nil
}

// BuildTypeExists checks if a build configuration exists
func (c *Client) BuildTypeExists(id string) bool {
	_, err := c.GetBuildType(id)
	return err == nil
}

// BuildStep represents a build step configuration
type BuildStep struct {
	ID         string       `json:"id,omitempty"`
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	Properties PropertyList `json:"properties,omitempty"`
}

// CreateBuildStep adds a build step to a build configuration
func (c *Client) CreateBuildStep(buildTypeID string, step BuildStep) error {
	body, err := json.Marshal(step)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/steps", buildTypeID)
	return c.doNoContent("POST", path, bytes.NewReader(body), "")
}

// GetVcsRootEntries returns the VCS root entries attached to a build configuration
func (c *Client) GetVcsRootEntries(buildTypeID string) (*VcsRootEntries, error) {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/vcs-root-entries", buildTypeID)

	var result VcsRootEntries
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SetBuildTypeSetting sets a build configuration setting
func (c *Client) SetBuildTypeSetting(buildTypeID, setting, value string) error {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/settings/%s", buildTypeID, setting)
	return c.doNoContent("PUT", path, strings.NewReader(value), "text/plain")
}
