package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// SettingsList represents a list of build configuration settings
type SettingsList struct {
	Count    int       `json:"count"`
	Property []Setting `json:"property"`
}

// Setting represents a single build configuration setting
type Setting struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// GetBuildTypeSettings returns all settings for a build configuration
func (c *Client) GetBuildTypeSettings(buildTypeID string) (*SettingsList, error) {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/settings", url.PathEscape(buildTypeID))

	var result SettingsList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}
	if result.Property == nil {
		result.Property = []Setting{} // non-nil so --json emits [] not null
	}

	return &result, nil
}

// GetBuildTypeSetting returns the raw plain-text value of a single build configuration setting
func (c *Client) GetBuildTypeSetting(buildTypeID, name string) (string, error) {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/settings/%s", url.PathEscape(buildTypeID), url.PathEscape(name))

	resp, err := c.doRequestWithAccept(c.ctx(), "GET", path, nil, "text/plain")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", c.handleErrorResponse(resp)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
