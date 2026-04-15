package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
)

type AgentPoolsOptions struct {
	Limit        int
	Skip         int
	ContinuePath string
	Fields       []string
}

// GetAgentPools returns all agent pools.
func (c *Client) GetAgentPools(opts AgentPoolsOptions) (*PoolList, error) {
	fields := opts.Fields
	if len(fields) == 0 {
		fields = PoolFields.Default
	}
	fieldsParam := paginatedFieldsParam("agentPool", fields)

	path := opts.ContinuePath
	if path != "" {
		var err error
		path, err = rewriteContinuationPath(path, opts.Limit, fieldsParam)
		if err != nil {
			return nil, err
		}
	} else {
		locator := NewLocator().
			AddIntDefault("count", opts.Limit, 100).
			AddInt("start", opts.Skip)
		path = fmt.Sprintf("/app/rest/agentPools?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))
	}

	var result PoolList
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetAgentPool returns details for a single pool
func (c *Client) GetAgentPool(id int) (*Pool, error) {
	fields := "id,name,maxAgents,projects(count,project(id,name)),agents(count,agent(id,name,connected,enabled,authorized))"
	path := fmt.Sprintf("/app/rest/agentPools/id:%d?fields=%s", id, url.QueryEscape(fields))

	var result Pool
	if err := c.get(path, &result); err != nil {
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
	return c.doNoContent("POST", path, bytes.NewReader(body), "")
}

// RemoveProjectFromPool removes a project from an agent pool
func (c *Client) RemoveProjectFromPool(poolID int, projectID string) error {
	path := fmt.Sprintf("/app/rest/agentPools/id:%d/projects/id:%s", poolID, projectID)
	return c.doNoContent("DELETE", path, nil, "")
}

// SetAgentPool moves an agent to a different pool
func (c *Client) SetAgentPool(agentID int, poolID int) error {
	path := fmt.Sprintf("/app/rest/agents/id:%d/pool", agentID)
	body, err := json.Marshal(Pool{ID: poolID})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	return c.doNoContent("PUT", path, bytes.NewReader(body), "")
}
