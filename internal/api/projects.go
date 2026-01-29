package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// ProjectsOptions represents options for listing projects
type ProjectsOptions struct {
	Parent string
	Limit  int
	Fields []string
}

// GetProjects returns a list of projects
func (c *Client) GetProjects(opts ProjectsOptions) (*ProjectList, error) {
	locator := NewLocator().
		Add("parentProject", opts.Parent).
		AddIntDefault("count", opts.Limit, 30)

	fields := opts.Fields
	if len(fields) == 0 {
		fields = ProjectFields.Default
	}
	fieldsParam := fmt.Sprintf("count,project(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/projects?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))

	var result ProjectList
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetProject returns a single project by ID
func (c *Client) GetProject(id string) (*Project, error) {
	path := fmt.Sprintf("/app/rest/projects/id:%s", id)

	var project Project
	if err := c.get(path, &project); err != nil {
		return nil, err
	}

	return &project, nil
}

// CreateProjectRequest represents a request to create a project
type CreateProjectRequest struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	ParentProjectID string `json:"parentProject,omitempty"`
}

// CreateProject creates a new project
func (c *Client) CreateProject(req CreateProjectRequest) (*Project, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var project Project
	if err := c.post("/app/rest/projects", bytes.NewReader(body), &project); err != nil {
		return nil, err
	}

	return &project, nil
}

// ProjectExists checks if a project exists
func (c *Client) ProjectExists(id string) bool {
	_, err := c.GetProject(id)
	return err == nil
}

// CreateSecureToken creates a new secure token for the given value in a project.
// Returns the scrambled token that can be used in configuration files as credentialsJSON:<token>.
// Requires EDIT_PROJECT permission.
func (c *Client) CreateSecureToken(projectID, value string) (string, error) {
	path := fmt.Sprintf("/app/rest/projects/%s/secure/tokens", projectID)

	resp, err := c.doRequestFull("POST", path, strings.NewReader(value), "text/plain", "text/plain")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", c.handleErrorResponse(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// GetSecureValue retrieves the original value for a secure token.
// Requires CHANGE_SERVER_SETTINGS permission (System Administrator only).
func (c *Client) GetSecureValue(projectID, token string) (string, error) {
	path := fmt.Sprintf("/app/rest/projects/%s/secure/values/%s", projectID, token)

	resp, err := c.doRequestWithAccept("GET", path, nil, "text/plain")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", c.handleErrorResponse(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// GetVersionedSettingsStatus returns the sync status of versioned settings for a project.
func (c *Client) GetVersionedSettingsStatus(projectID string) (*VersionedSettingsStatus, error) {
	path := fmt.Sprintf("/app/rest/projects/%s/versionedSettings/status", projectID)

	var status VersionedSettingsStatus
	if err := c.get(path, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// GetVersionedSettingsConfig returns the versioned settings configuration for a project.
func (c *Client) GetVersionedSettingsConfig(projectID string) (*VersionedSettingsConfig, error) {
	path := fmt.Sprintf("/app/rest/projects/%s/versionedSettings/config", projectID)

	var config VersionedSettingsConfig
	if err := c.get(path, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
