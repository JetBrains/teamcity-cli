package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// BuildTypesOptions represents options for listing build configurations
type BuildTypesOptions struct {
	Project    string
	VcsRootURL string // server-side substring filter on each VCS root's `url` property
	Limit      int
	Fields     []string
}

// GetBuildTypes returns a list of build configurations, following pagination; the bool is true when a finite limit capped the result.
func (c *Client) GetBuildTypes(opts BuildTypesOptions) (*BuildTypeList, bool, error) {
	locator := NewLocator().
		Add("affectedProject", opts.Project).
		AddInt("count", pageCount(opts.Limit))
	if opts.VcsRootURL != "" {
		locator.AddLocator("vcsRoot", NewLocator().
			AddLocator("property", NewLocator().
				Add("name", "url").
				Add("value", opts.VcsRootURL).
				Add("matchType", "contains")))
	}

	fields := opts.Fields
	if len(fields) == 0 {
		fields = BuildTypeFields.Default
	}
	fieldsParam := fmt.Sprintf("count,nextHref,buildType(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/buildTypes?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))

	buildTypes, truncated, err := collectPages(c, path, opts.Limit, func(p string) ([]BuildType, string, error) {
		var page BuildTypeList
		if err := c.get(c.ctx(), p, &page); err != nil {
			return nil, "", err
		}
		return page.BuildTypes, page.NextHref, nil
	})
	if err != nil {
		return nil, false, err
	}

	return &BuildTypeList{Count: len(buildTypes), BuildTypes: buildTypes}, truncated, nil
}

// GetBuildType returns a single build configuration by ID
func (c *Client) GetBuildType(id string) (*BuildType, error) {
	path := "/app/rest/buildTypes/id:" + id

	var buildType BuildType
	if err := c.get(c.ctx(), path, &buildType); err != nil {
		return nil, err
	}

	return &buildType, nil
}

// SetBuildTypePaused sets the paused state of a build configuration
func (c *Client) SetBuildTypePaused(id string, paused bool) error {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/paused", id)

	resp, err := c.doRequestFull(c.ctx(), "PUT", path, strings.NewReader(strconv.FormatBool(paused)), "text/plain", "text/plain")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// CreateBuildTypeRequest represents a request to create a build configuration; set Templates to create it from a template.
type CreateBuildTypeRequest struct {
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name"`
	Project   *ProjectRef    `json:"project,omitempty"`
	Templates *BuildTypeList `json:"templates,omitempty"`
}

// CreateBuildType creates a new build configuration in a project. It posts a full
// BuildType to /app/rest/buildTypes (not the project-scoped NewBuildTypeDescription
// endpoint, which silently ignores the templates field).
func (c *Client) CreateBuildType(projectID string, req CreateBuildTypeRequest) (*BuildType, error) {
	req.Project = &ProjectRef{ID: projectID}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var buildType BuildType
	if err := c.post(c.ctx(), "/app/rest/buildTypes", bytes.NewReader(body), &buildType); err != nil {
		return nil, err
	}

	return &buildType, nil
}

// BuildTypeExists checks if a build configuration exists
func (c *Client) BuildTypeExists(id string) bool {
	_, err := c.GetBuildType(id)
	return err == nil
}

// BuildStep represents a build step configuration
type BuildStep struct {
	ID         string       `json:"id,omitempty"`
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	Disabled   bool         `json:"disabled,omitempty"`
	Properties PropertyList `json:"properties"`
}

// BuildStepList represents the build steps of a build configuration
type BuildStepList struct {
	Count int         `json:"count"`
	Step  []BuildStep `json:"step"`
}

const buildStepFields = "count,step(id,name,type,disabled,properties(property(name,value)))"

// GetBuildSteps returns the build steps of a build configuration
func (c *Client) GetBuildSteps(buildTypeID string) (*BuildStepList, error) {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/steps?fields=%s", buildTypeID, url.QueryEscape(buildStepFields))

	var result BuildStepList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}
	if result.Step == nil {
		result.Step = []BuildStep{} // non-nil so --json emits [] not null
	}

	return &result, nil
}

// GetBuildStep returns a single build step by ID
func (c *Client) GetBuildStep(buildTypeID, stepID string) (*BuildStep, error) {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/steps/%s", buildTypeID, stepID)

	var step BuildStep
	if err := c.get(c.ctx(), path, &step); err != nil {
		return nil, err
	}

	return &step, nil
}

// CreateBuildStep adds a build step to a build configuration and returns the created step
func (c *Client) CreateBuildStep(buildTypeID string, step BuildStep) (*BuildStep, error) {
	body, err := json.Marshal(step)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/steps", buildTypeID)

	var created BuildStep
	if err := c.post(c.ctx(), path, bytes.NewReader(body), &created); err != nil {
		return nil, err
	}

	return &created, nil
}

// DeleteBuildStep removes a build step from a build configuration
func (c *Client) DeleteBuildStep(buildTypeID, stepID string) error {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/steps/%s", buildTypeID, stepID)
	return c.doNoContent(c.ctx(), "DELETE", path, nil, "")
}

// GetSnapshotDependencies returns the snapshot dependencies for a build configuration
func (c *Client) GetSnapshotDependencies(buildTypeID string) (*SnapshotDependencyList, error) {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/snapshot-dependencies?fields=count,snapshot-dependency(id,source-buildType(id,name,projectId))", buildTypeID)

	var result SnapshotDependencyList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetDependentBuildTypes returns build types that have a snapshot dependency on the given build type.
func (c *Client) GetDependentBuildTypes(buildTypeID string) (*BuildTypeList, error) {
	path := fmt.Sprintf("/app/rest/buildTypes?locator=snapshotDependency:(from:(id:%s),recursive:false)&fields=count,buildType(id,name,projectId)", buildTypeID)

	var result BuildTypeList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetVcsRootEntries returns the VCS root entries attached to a build configuration
func (c *Client) GetVcsRootEntries(buildTypeID string) (*VcsRootEntries, error) {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/vcs-root-entries", buildTypeID)

	var result VcsRootEntries
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SetBuildTypeSetting sets a build configuration setting
func (c *Client) SetBuildTypeSetting(buildTypeID, setting, value string) error {
	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/settings/%s", buildTypeID, setting)
	return c.doNoContent(c.ctx(), "PUT", path, strings.NewReader(value), "text/plain")
}
