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
	fieldsParam := fmt.Sprintf("count,agent(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/agents?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))

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

// agentDetailFields is the fields parameter used for agent detail requests
const agentDetailFields = "id,name,typeId,connected,enabled,authorized,href,webUrl,pool(id,name),build(id,number,status,buildType(id,name))"

// GetAgent returns details for a single agent
func (c *Client) GetAgent(id int) (*Agent, error) {
	path := fmt.Sprintf("/app/rest/agents/id:%d?fields=%s", id, url.QueryEscape(agentDetailFields))

	var result Agent
	if err := c.get(path, &result); err != nil {
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

// RebootAgent requests a reboot of the specified agent.
// If afterBuild is true, the agent will reboot after the current build finishes.
// This uses the web UI endpoint as there is no REST API for agent reboot.
func (c *Client) RebootAgent(ctx context.Context, id int, afterBuild bool) error {
	formData := url.Values{}
	formData.Set("agent", fmt.Sprintf("%d", id))
	if afterBuild {
		formData.Set("rebootAfterBuild", "true")
	}

	endpoint := fmt.Sprintf("%s/remoteAccess/reboot.html", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.setAuth(req)

	debugLogRequest(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return &NetworkError{URL: c.BaseURL, Cause: err}
	}
	defer func() { _ = resp.Body.Close() }()

	debugLogResponse(resp)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusFound:
		return nil
	case http.StatusUnauthorized:
		return ErrAuthentication
	case http.StatusForbidden:
		return &PermissionError{Action: "reboot agent"}
	case http.StatusNotFound:
		return &NotFoundError{Resource: "agent", ID: fmt.Sprintf("%d", id)}
	default:
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
}
