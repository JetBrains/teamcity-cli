package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// ParameterList represents a list of parameters
type ParameterList struct {
	Count    int         `json:"count"`
	Property []Parameter `json:"property"`
}

// Parameter represents a TeamCity parameter
type Parameter struct {
	Name  string         `json:"name"`
	Value string         `json:"value"`
	Type  *ParameterType `json:"type,omitempty"`
}

// ParameterType represents parameter type info
type ParameterType struct {
	RawValue string `json:"rawValue,omitempty"`
}

func (c *Client) getParameters(basePath string) (*ParameterList, error) {
	path := basePath + "/parameters"

	var params ParameterList
	if err := c.get(path, &params); err != nil {
		return nil, err
	}

	return &params, nil
}

func (c *Client) getParameter(basePath, name string) (*Parameter, error) {
	path := fmt.Sprintf("%s/parameters/%s", basePath, name)

	var param Parameter
	if err := c.get(path, &param); err != nil {
		return nil, err
	}

	return &param, nil
}

func (c *Client) setParameter(basePath, name, value string, secure bool) error {
	path := fmt.Sprintf("%s/parameters/%s", basePath, name)

	param := Parameter{
		Name:  name,
		Value: value,
	}

	if secure {
		param.Type = &ParameterType{RawValue: "password"}
	}

	body, err := json.Marshal(param)
	if err != nil {
		return fmt.Errorf("failed to marshal parameter: %w", err)
	}

	return c.doNoContent("PUT", path, bytes.NewReader(body), "")
}

func (c *Client) deleteParameter(basePath, name string) error {
	path := fmt.Sprintf("%s/parameters/%s", basePath, name)
	return c.doNoContent("DELETE", path, nil, "")
}

// GetProjectParameters returns parameters for a project
func (c *Client) GetProjectParameters(projectID string) (*ParameterList, error) {
	return c.getParameters(fmt.Sprintf("/app/rest/projects/id:%s", projectID))
}

// GetProjectParameter returns a specific parameter for a project
func (c *Client) GetProjectParameter(projectID, name string) (*Parameter, error) {
	return c.getParameter(fmt.Sprintf("/app/rest/projects/id:%s", projectID), name)
}

// SetProjectParameter sets a parameter for a project
func (c *Client) SetProjectParameter(projectID, name, value string, secure bool) error {
	return c.setParameter(fmt.Sprintf("/app/rest/projects/id:%s", projectID), name, value, secure)
}

// DeleteProjectParameter deletes a parameter from a project
func (c *Client) DeleteProjectParameter(projectID, name string) error {
	return c.deleteParameter(fmt.Sprintf("/app/rest/projects/id:%s", projectID), name)
}

// GetBuildTypeParameters returns parameters for a build configuration
func (c *Client) GetBuildTypeParameters(buildTypeID string) (*ParameterList, error) {
	return c.getParameters(fmt.Sprintf("/app/rest/buildTypes/id:%s", buildTypeID))
}

// GetBuildTypeParameter returns a specific parameter for a build configuration
func (c *Client) GetBuildTypeParameter(buildTypeID, name string) (*Parameter, error) {
	return c.getParameter(fmt.Sprintf("/app/rest/buildTypes/id:%s", buildTypeID), name)
}

// SetBuildTypeParameter sets a parameter for a build configuration
func (c *Client) SetBuildTypeParameter(buildTypeID, name, value string, secure bool) error {
	return c.setParameter(fmt.Sprintf("/app/rest/buildTypes/id:%s", buildTypeID), name, value, secure)
}

// DeleteBuildTypeParameter deletes a parameter from a build configuration
func (c *Client) DeleteBuildTypeParameter(buildTypeID, name string) error {
	return c.deleteParameter(fmt.Sprintf("/app/rest/buildTypes/id:%s", buildTypeID), name)
}

// GetParameterValue returns just the raw value of a parameter
func (c *Client) GetParameterValue(path string) (string, error) {
	resp, err := c.doRequestWithAccept("GET", path, nil, "text/plain")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", c.handleErrorResponse(resp)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
