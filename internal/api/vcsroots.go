package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
)

// VcsRoot represents a TeamCity VCS root
type VcsRoot struct {
	ID         string       `json:"id,omitempty"`
	Name       string       `json:"name,omitempty"`
	VcsName    string       `json:"vcsName,omitempty"` // e.g. "jetbrains.git", "perforce"
	ProjectID  string       `json:"projectId,omitempty"`
	Href       string       `json:"href,omitempty"`
	Properties PropertyList `json:"properties,omitempty"`
	Project    *Project     `json:"project,omitempty"`
}

// VcsRootList represents a list of VCS roots
type VcsRootList struct {
	Count    int       `json:"count"`
	VcsRoots []VcsRoot `json:"vcs-root"`
}

// VcsRootOptions represents options for listing VCS roots
type VcsRootOptions struct {
	Project string
	Limit   int
}

// GetVcsRoots returns a list of VCS roots
func (c *Client) GetVcsRoots(opts VcsRootOptions) (*VcsRootList, error) {
	locator := NewLocator().
		Add("project", opts.Project).
		AddIntDefault("count", opts.Limit, 30)

	path := fmt.Sprintf("/app/rest/vcs-roots?locator=%s", locator.Encode())

	var result VcsRootList
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetVcsRoot returns a single VCS root by ID
func (c *Client) GetVcsRoot(id string) (*VcsRoot, error) {
	path := fmt.Sprintf("/app/rest/vcs-roots/id:%s", url.PathEscape(id))

	var vcsRoot VcsRoot
	if err := c.get(path, &vcsRoot); err != nil {
		return nil, err
	}

	return &vcsRoot, nil
}

// CreateVcsRootRequest represents a request to create a VCS root
type CreateVcsRootRequest struct {
	ID         string       `json:"id,omitempty"`
	Name       string       `json:"name"`
	VcsName    string       `json:"vcsName"` // "jetbrains.git", "perforce"
	ProjectID  string       `json:"project,omitempty"`
	Properties PropertyList `json:"properties,omitempty"`
}

// createVcsRootPayload is the JSON format expected by the API
type createVcsRootPayload struct {
	ID         string       `json:"id,omitempty"`
	Name       string       `json:"name"`
	VcsName    string       `json:"vcsName"`
	Project    *ProjectRef  `json:"project,omitempty"`
	Properties PropertyList `json:"properties,omitempty"`
}

// ProjectRef is a reference to a project by ID
type ProjectRef struct {
	ID string `json:"id"`
}

// CreateVcsRoot creates a new VCS root
func (c *Client) CreateVcsRoot(req CreateVcsRootRequest) (*VcsRoot, error) {
	payload := createVcsRootPayload{
		ID:         req.ID,
		Name:       req.Name,
		VcsName:    req.VcsName,
		Properties: req.Properties,
	}
	if req.ProjectID != "" {
		payload.Project = &ProjectRef{ID: req.ProjectID}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var vcsRoot VcsRoot
	if err := c.post("/app/rest/vcs-roots", bytes.NewReader(body), &vcsRoot); err != nil {
		return nil, err
	}

	return &vcsRoot, nil
}

// DeleteVcsRoot deletes a VCS root by ID
func (c *Client) DeleteVcsRoot(id string) error {
	path := fmt.Sprintf("/app/rest/vcs-roots/id:%s", url.PathEscape(id))
	return c.doNoContent("DELETE", path, nil, "")
}

// VcsRootExists checks if a VCS root exists
func (c *Client) VcsRootExists(id string) bool {
	_, err := c.GetVcsRoot(id)
	return err == nil
}

// AttachVcsRoot attaches a VCS root to a build configuration
func (c *Client) AttachVcsRoot(buildTypeID string, vcsRootID string) error {
	payload := struct {
		ID      string `json:"id"`
		VcsRoot struct {
			ID string `json:"id"`
		} `json:"vcs-root"`
	}{
		ID: vcsRootID,
	}
	payload.VcsRoot.ID = vcsRootID

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/app/rest/buildTypes/id:%s/vcs-root-entries", buildTypeID)
	return c.doNoContent("POST", path, bytes.NewReader(body), "")
}

// Helper: creates a property list for a Perforce VCS root
func NewPerforceVcsRootProperties(port, user, password, stream string) PropertyList {
	props := []Property{
		{Name: "port", Value: port},
		{Name: "user", Value: user},
	}
	if password != "" {
		props = append(props, Property{Name: "secure:passwd", Value: password})
	}
	if stream != "" {
		props = append(props, Property{Name: "stream", Value: stream},
			Property{Name: "use-client", Value: "false"})
	}
	return PropertyList{Property: props}
}

// Helper: creates a property list for a Git VCS root
func NewGitVcsRootProperties(repoURL, branch string) PropertyList {
	props := []Property{
		{Name: "url", Value: repoURL},
	}
	if branch != "" {
		props = append(props, Property{Name: "branch", Value: branch})
	}
	return PropertyList{Property: props}
}
