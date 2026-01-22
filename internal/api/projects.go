package api

import (
	"fmt"
	"io"
	"strings"
)

// ProjectsOptions represents options for listing projects
type ProjectsOptions struct {
	Parent string
	Limit  int
}

// GetProjects returns a list of projects
func (c *Client) GetProjects(opts ProjectsOptions) (*ProjectList, error) {
	locator := NewLocator().
		Add("parentProject", opts.Parent).
		AddIntDefault("count", opts.Limit, 30)

	path := fmt.Sprintf("/app/rest/projects?locator=%s", locator.Encode())

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

// CreateSecureToken creates a new secure token for the given value in a project.
// Returns the scrambled token that can be used in configuration files as credentialsJSON:<token>.
// Requires EDIT_PROJECT permission.
func (c *Client) CreateSecureToken(projectID, value string) (string, error) {
	path := fmt.Sprintf("/app/rest/projects/%s/secure/tokens", projectID)

	resp, err := c.doRequestFull("POST", path, strings.NewReader(value), "text/plain", "text/plain")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

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
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", c.handleErrorResponse(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
