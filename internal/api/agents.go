package api

import (
	"fmt"
	"net/url"
	"strings"
)

// AgentsOptions represents options for listing agents
type AgentsOptions struct {
	Authorized bool   // Filter by authorization status
	Connected  bool   // Filter by connection status
	Enabled    bool   // Filter by enabled status
	Pool       string // Filter by pool name
	Limit      int
}

// GetAgents returns a list of agents
func (c *Client) GetAgents(opts AgentsOptions) (*AgentList, error) {
	locator := NewLocator()

	if opts.Authorized {
		locator.Add("authorized", "true")
	} else {
		locator.Add("authorized", "any")
	}

	if opts.Connected {
		locator.Add("connected", "true")
	}
	if opts.Enabled {
		locator.Add("enabled", "true")
	}
	if opts.Pool != "" {
		locator.Add("pool", opts.Pool)
	}
	locator.AddIntDefault("count", opts.Limit, 100)

	fields := "count,agent(id,name,connected,enabled,authorized,pool(id,name))"
	path := fmt.Sprintf("/app/rest/agents?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fields))

	var result AgentList
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// AuthorizeAgent sets the authorized status of an agent
func (c *Client) AuthorizeAgent(id int, authorized bool) error {
	path := fmt.Sprintf("/app/rest/agents/id:%d/authorized", id)
	value := "false"
	if authorized {
		value = "true"
	}
	return c.doNoContent("PUT", path, strings.NewReader(value), "text/plain")
}

// GetAgent returns details for a single agent
func (c *Client) GetAgent(id int) (*Agent, error) {
	fields := "id,name,typeId,connected,enabled,authorized,href,webUrl,pool(id,name),build(id,number,status,buildType(id,name))"
	path := fmt.Sprintf("/app/rest/agents/id:%d?fields=%s", id, url.QueryEscape(fields))

	var result Agent
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// EnableAgent sets the enabled status of an agent
func (c *Client) EnableAgent(id int, enabled bool) error {
	path := fmt.Sprintf("/app/rest/agents/id:%d/enabled", id)
	value := "false"
	if enabled {
		value = "true"
	}
	return c.doNoContent("PUT", path, strings.NewReader(value), "text/plain")
}

// GetAgentCompatibleBuildTypes returns build types compatible with an agent
func (c *Client) GetAgentCompatibleBuildTypes(id int) (*BuildTypeList, error) {
	fields := "count,buildType(id,name,projectName,projectId)"
	path := fmt.Sprintf("/app/rest/agents/id:%d/compatibleBuildTypes?fields=%s", id, url.QueryEscape(fields))

	var result BuildTypeList
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetAgentIncompatibleBuildTypes returns build types incompatible with an agent and reasons
func (c *Client) GetAgentIncompatibleBuildTypes(id int) (*CompatibilityList, error) {
	fields := "count,compatibility(buildType(id,name,projectName),incompatibleReasons(reason))"
	path := fmt.Sprintf("/app/rest/agents/id:%d/incompatibleBuildTypes?fields=%s", id, url.QueryEscape(fields))

	var result CompatibilityList
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
