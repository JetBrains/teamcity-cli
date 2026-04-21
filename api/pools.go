package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// GetAgentPools returns all agent pools, automatically following pagination.
func (c *Client) GetAgentPools(requestedFields []string) (*PoolList, error) {
	fields := requestedFields
	if len(fields) == 0 {
		fields = PoolFields.Default
	}
	fieldsParam := fmt.Sprintf("count,nextHref,agentPool(%s)", ToAPIFields(fields))
	path := "/app/rest/agentPools?fields=" + url.QueryEscape(fieldsParam)

	pools, err := collectPages(c, path, 0, func(p string) ([]Pool, string, error) {
		var page PoolList
		if err := c.get(context.Background(), p, &page); err != nil {
			return nil, "", err
		}
		return page.Pools, page.NextHref, nil
	})
	if err != nil {
		return nil, err
	}

	return &PoolList{Count: len(pools), Pools: pools}, nil
}

// GetAgentPool returns details for a single pool
func (c *Client) GetAgentPool(id int) (*Pool, error) {
	fields := "id,name,maxAgents,projects(count,project(id,name)),agents(count,agent(id,name,connected,enabled,authorized))"
	path := fmt.Sprintf("/app/rest/agentPools/id:%d?fields=%s", id, url.QueryEscape(fields))

	var result Pool
	if err := c.get(context.Background(), path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// AddProjectToPool assigns a project to an agent pool
func (c *Client) AddProjectToPool(poolID int, projectID string) error {
	path := fmt.Sprintf("/app/rest/agentPools/id:%d/projects", poolID)
	body, err := json.Marshal(Project{ID: projectID})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	return c.doNoContent(context.Background(), "POST", path, bytes.NewReader(body), "")
}

// RemoveProjectFromPool removes a project from an agent pool
func (c *Client) RemoveProjectFromPool(poolID int, projectID string) error {
	path := fmt.Sprintf("/app/rest/agentPools/id:%d/projects/id:%s", poolID, projectID)
	return c.doNoContent(context.Background(), "DELETE", path, nil, "")
}

// SetAgentPool moves an agent to a different pool
func (c *Client) SetAgentPool(agentID int, poolID int) error {
	path := fmt.Sprintf("/app/rest/agents/id:%d/pool", agentID)
	body, err := json.Marshal(Pool{ID: poolID})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	return c.doNoContent(context.Background(), "PUT", path, bytes.NewReader(body), "")
}
