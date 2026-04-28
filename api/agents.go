package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// AgentsOptions represents options for listing agents
type AgentsOptions struct {
	Authorized bool   // Filter by authorization status
	Connected  bool   // Filter by connection status
	Enabled    bool   // Filter by enabled status
	Pool       string // Filter by pool name
	Limit      int
	Fields     []string // Fields to return (uses AgentFields.Default if empty)
}

// GetAgents returns a list of agents, automatically following pagination.
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
		if _, err := strconv.Atoi(opts.Pool); err == nil {
			locator.AddRaw("pool", "(id:"+opts.Pool+")")
		} else {
			locator.AddRaw("pool", "(name:"+opts.Pool+")")
		}
	}
	locator.AddIntDefault("count", opts.Limit, 100)

	fields := opts.Fields
	if len(fields) == 0 {
		fields = AgentFields.Default
	}
	fieldsParam := fmt.Sprintf("count,nextHref,agent(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/agents?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))

	agents, err := collectPages(c, path, opts.Limit, func(p string) ([]Agent, string, error) {
		var page AgentList
		if err := c.get(c.ctx(), p, &page); err != nil {
			return nil, "", err
		}
		return page.Agents, page.NextHref, nil
	})
	if err != nil {
		return nil, err
	}

	return &AgentList{Count: len(agents), Agents: agents}, nil
}

// AuthorizeAgent sets the authorized status of an agent
func (c *Client) AuthorizeAgent(id int, authorized bool) error {
	path := fmt.Sprintf("/app/rest/agents/id:%d/authorized", id)
	value := "false"
	if authorized {
		value = "true"
	}
	return c.doNoContent(c.ctx(), "PUT", path, strings.NewReader(value), "text/plain")
}

// agentDetailFields is the fields parameter used for agent detail requests
const agentDetailFields = "id,name,typeId,connected,enabled,authorized,href,webUrl,pool(id,name),build(id,number,status,buildType(id,name))"

// GetAgent returns details for a single agent
func (c *Client) GetAgent(id int) (*Agent, error) {
	path := fmt.Sprintf("/app/rest/agents/id:%d?fields=%s", id, url.QueryEscape(agentDetailFields))

	var result Agent
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetAgentByName returns details for an agent by name.
// PathEscape is sufficient here: TeamCity prohibits colons and commas in agent names
// (they conflict with locator syntax), so we only need to escape path-unsafe characters.
func (c *Client) GetAgentByName(name string) (*Agent, error) {
	path := fmt.Sprintf("/app/rest/agents/name:%s?fields=%s", url.PathEscape(name), url.QueryEscape(agentDetailFields))

	var result Agent
	if err := c.get(c.ctx(), path, &result); err != nil {
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
	return c.doNoContent(c.ctx(), "PUT", path, strings.NewReader(value), "text/plain")
}

// GetAgentCompatibleBuildTypes returns build types compatible with an agent
func (c *Client) GetAgentCompatibleBuildTypes(id int) (*BuildTypeList, error) {
	fields := "count,buildType(id,name,projectName,projectId)"
	path := fmt.Sprintf("/app/rest/agents/id:%d/compatibleBuildTypes?fields=%s", id, url.QueryEscape(fields))

	var result BuildTypeList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetAgentIncompatibleBuildTypes returns build types incompatible with an agent and reasons
func (c *Client) GetAgentIncompatibleBuildTypes(id int) (*CompatibilityList, error) {
	fields := "count,compatibility(buildType(id,name,projectName),incompatibleReasons(reason),unmetRequirements(description))"
	path := fmt.Sprintf("/app/rest/agents/id:%d/incompatibleBuildTypes?fields=%s", id, url.QueryEscape(fields))

	var result CompatibilityList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

const buildAgentsFields = "count,agent(id,name,pool(id,name),connected,enabled,authorized)"

// GetBuildCompatibleAgents returns agents compatible with the given build, including disconnected ones.
func (c *Client) GetBuildCompatibleAgents(buildID int) (*AgentList, error) {
	locator := fmt.Sprintf("compatible:(build:(id:%d)),defaultFilter:false", buildID)
	path := fmt.Sprintf("/app/rest/agents?locator=%s&fields=%s", url.QueryEscape(locator), url.QueryEscape(buildAgentsFields))

	var result AgentList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetBuildIncompatibleAgents returns agents incompatible with the given build.
func (c *Client) GetBuildIncompatibleAgents(buildID int) (*AgentList, error) {
	locator := fmt.Sprintf("incompatible:(build:(id:%d)),defaultFilter:false", buildID)
	path := fmt.Sprintf("/app/rest/agents?locator=%s&fields=%s", url.QueryEscape(locator), url.QueryEscape(buildAgentsFields))

	var result AgentList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAgentBuildTypeCompatibility finds the (agent, buildType) entry in incompatibleBuildTypes, scanning at most maxScan.
func (c *Client) GetAgentBuildTypeCompatibility(agentID int, buildTypeID string, maxScan int) (*Compatibility, error) {
	if maxScan <= 0 {
		maxScan = 5000
	}
	fields := "count,compatibility(buildType(id),compatible,incompatibleReasons(reason),unmetRequirements(description))"
	locator := fmt.Sprintf("id:%s,count:%d", buildTypeID, maxScan)
	path := fmt.Sprintf("/app/rest/agents/id:%d/incompatibleBuildTypes?locator=%s&fields=%s",
		agentID, url.QueryEscape(locator), url.QueryEscape(fields))

	var result CompatibilityList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}
	for i := range result.Compatibility {
		bt := result.Compatibility[i].BuildType
		if bt != nil && bt.ID == buildTypeID {
			return &result.Compatibility[i], nil
		}
	}
	return nil, nil
}

// RebootAgent requests a reboot of the specified agent.
// If afterBuild is true, the agent will reboot after the current build finishes.
// This uses the web UI endpoint as there is no REST API for agent reboot.
func (c *Client) RebootAgent(ctx context.Context, id int, afterBuild bool) error {
	if c.ReadOnly {
		return fmt.Errorf("%w: POST /remoteAccess/reboot.html", ErrReadOnly)
	}

	formData := url.Values{}
	formData.Set("agent", strconv.Itoa(id))
	if afterBuild {
		formData.Set("rebootAfterBuild", "true")
	}

	endpoint := c.BaseURL + "/remoteAccess/reboot.html"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.setAuth(req)
	c.applyStandardHeaders(req)

	c.debugLogRequest(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return &NetworkError{URL: c.BaseURL, Cause: err}
	}
	defer func() { _ = resp.Body.Close() }()

	c.debugLogResponse(resp)

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusFound {
		return nil
	}
	return ErrorFromResponse(resp)
}
