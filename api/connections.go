package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
)

// GetProjectConnections returns OAuth/connection features for a project
func (c *Client) GetProjectConnections(projectID string) (*ProjectFeatureList, error) {
	fields := url.QueryEscape("projectFeature(id,type,properties(property(name,value)))")
	path := fmt.Sprintf("/app/rest/projects/id:%s/projectFeatures?locator=type:OAuthProvider&fields=%s", projectID, fields)

	var result ProjectFeatureList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateProjectFeature creates a new project feature (e.g., OAuth connection).
func (c *Client) CreateProjectFeature(projectID string, feat ProjectFeature) (*ProjectFeature, error) {
	body, err := json.Marshal(feat)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/app/rest/projects/id:%s/projectFeatures", projectID)
	var result ProjectFeature
	if err := c.post(c.ctx(), path, bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteProjectFeature removes a project feature by id.
func (c *Client) DeleteProjectFeature(projectID, featureID string) error {
	path := fmt.Sprintf("/app/rest/projects/id:%s/projectFeatures/id:%s", projectID, featureID)
	return c.doNoContent(c.ctx(), "DELETE", path, nil, "")
}
