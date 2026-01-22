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
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  *ParameterType `json:"type,omitempty"`
}

// ParameterType represents parameter type info
type ParameterType struct {
	RawValue string `json:"rawValue,omitempty"`
}

// GetProjectParameters returns parameters for a project
func (c *Client) GetProjectParameters(projectID string) (*ParameterList, error) {
	path := fmt.Sprintf("/app/rest/projects/id:%s/parameters", projectID)

	var params ParameterList
	if err := c.get(path, &params); err != nil {
		return nil, err
	}

	return &params, nil
}

// GetProjectParameter returns a specific parameter for a project
func (c *Client) GetProjectParameter(projectID, name string) (*Parameter, error) {
	path := fmt.Sprintf("/app/rest/projects/id:%s/parameters/%s", projectID, name)

	var param Parameter
	if err := c.get(path, &param); err != nil {
		return nil, err
	}

	return &param, nil
}

// SetProjectParameter sets a parameter for a project
func (c *Client) SetProjectParameter(projectID, name, value string, secure bool) error {
	path := fmt.Sprintf("/app/rest/projects/id:%s/parameters/%s", projectID, name)

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

	resp, err := c.doRequest("PUT", path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// DeleteProjectParameter deletes a parameter from a project
func (c *Client) DeleteProjectParameter(projectID, name string) error {
	path := fmt.Sprintf("/app/rest/projects/id:%s/parameters/%s", projectID, name)

	resp, err := c.doRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// GetBuildTypeParameters returns parameters for a build configuration
func (c *Client) GetBuildTypeParameters(buildTypeID string) (*ParameterList, error) {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/parameters", buildTypeID)

	var params ParameterList
	if err := c.get(path, &params); err != nil {
		return nil, err
	}

	return &params, nil
}

// GetBuildTypeParameter returns a specific parameter for a build configuration
func (c *Client) GetBuildTypeParameter(buildTypeID, name string) (*Parameter, error) {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/parameters/%s", buildTypeID, name)

	var param Parameter
	if err := c.get(path, &param); err != nil {
		return nil, err
	}

	return &param, nil
}

// SetBuildTypeParameter sets a parameter for a build configuration
func (c *Client) SetBuildTypeParameter(buildTypeID, name, value string, secure bool) error {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/parameters/%s", buildTypeID, name)

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

	resp, err := c.doRequest("PUT", path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// DeleteBuildTypeParameter deletes a parameter from a build configuration
func (c *Client) DeleteBuildTypeParameter(buildTypeID, name string) error {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/parameters/%s", buildTypeID, name)

	resp, err := c.doRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// GetParameterValue returns just the raw value of a parameter
func (c *Client) GetParameterValue(path string) (string, error) {
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", c.handleErrorResponse(resp)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
