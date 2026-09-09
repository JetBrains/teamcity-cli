package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
)

// GetProjectConnections returns connections declared in a project and its ancestors.
func (c *Client) GetProjectConnections(projectID string) (*ProjectFeatureList, error) {
	result := &ProjectFeatureList{ProjectFeature: []ProjectFeature{}}
	seenProjects := map[string]bool{}
	seenFeatures := map[string]bool{}
	for projectID != "" {
		if seenProjects[projectID] {
			return nil, fmt.Errorf("cycle in project hierarchy at %s", projectID)
		}
		seenProjects[projectID] = true
		fields := url.QueryEscape("projectFeature(id,type,properties(property(name,value)))")
		path := fmt.Sprintf("/app/rest/projects/id:%s/projectFeatures?locator=type:OAuthProvider&fields=%s", url.PathEscape(projectID), fields)
		var own ProjectFeatureList
		if err := c.get(c.ctx(), path, &own); err != nil {
			return nil, err
		}
		for _, feature := range own.ProjectFeature {
			if !seenFeatures[feature.ID] {
				result.ProjectFeature = append(result.ProjectFeature, feature)
				seenFeatures[feature.ID] = true
			}
		}
		if projectID == "_Root" {
			break
		}
		var project Project
		path = fmt.Sprintf("/app/rest/projects/id:%s?fields=id,parentProjectId", url.PathEscape(projectID))
		if err := c.get(c.ctx(), path, &project); err != nil {
			return nil, fmt.Errorf("failed to resolve parent project of %s: %w", projectID, err)
		}
		projectID = project.ParentProjectID
	}
	result.Count = len(result.ProjectFeature)
	return result, nil
}

// CreateProjectFeature creates a new project feature (e.g., OAuth connection).
func (c *Client) CreateProjectFeature(projectID string, feat ProjectFeature) (*ProjectFeature, error) {
	body, err := json.Marshal(feat)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/app/rest/projects/id:%s/projectFeatures", url.PathEscape(projectID))
	var result ProjectFeature
	if err := c.post(c.ctx(), path, bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteProjectFeature removes a project feature by id.
func (c *Client) DeleteProjectFeature(projectID, featureID string) error {
	path := fmt.Sprintf("/app/rest/projects/id:%s/projectFeatures/id:%s", url.PathEscape(projectID), url.PathEscape(featureID))
	return c.doNoContent(c.ctx(), "DELETE", path, nil, "")
}
