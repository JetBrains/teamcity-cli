package api

import (
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
