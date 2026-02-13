package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
)

type VcsRoot struct {
	ID         string       `json:"id,omitempty"`
	Name       string       `json:"name,omitempty"`
	VcsName    string       `json:"vcsName,omitempty"` // "jetbrains.git", "perforce"
	ProjectID  string       `json:"projectId,omitempty"`
	Href       string       `json:"href,omitempty"`
	Properties PropertyList `json:"properties,omitempty"`
	Project    *Project     `json:"project,omitempty"`
}

type VcsRootList struct {
	Count    int       `json:"count"`
	VcsRoots []VcsRoot `json:"vcs-root"`
}

type VcsRootOptions struct {
	Project string
	Limit   int
}

func (c *Client) GetVcsRoots(opts VcsRootOptions) (*VcsRootList, error) {
	locator := NewLocator().
		Add("project", opts.Project).
		AddIntDefault("count", opts.Limit, 30)

	var result VcsRootList
	if err := c.get(fmt.Sprintf("/app/rest/vcs-roots?locator=%s", locator.Encode()), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetVcsRoot(id string) (*VcsRoot, error) {
	var vcsRoot VcsRoot
	if err := c.get(fmt.Sprintf("/app/rest/vcs-roots/id:%s", url.PathEscape(id)), &vcsRoot); err != nil {
		return nil, err
	}
	return &vcsRoot, nil
}

type CreateVcsRootRequest struct {
	ID         string       `json:"id,omitempty"`
	Name       string       `json:"name"`
	VcsName    string       `json:"vcsName"` // "jetbrains.git", "perforce"
	ProjectID  string       `json:"project,omitempty"`
	Properties PropertyList `json:"properties,omitempty"`
}

type createVcsRootPayload struct {
	ID         string       `json:"id,omitempty"`
	Name       string       `json:"name"`
	VcsName    string       `json:"vcsName"`
	Project    *ProjectRef  `json:"project,omitempty"`
	Properties PropertyList `json:"properties,omitempty"`
}

type ProjectRef struct {
	ID string `json:"id"`
}

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

func (c *Client) DeleteVcsRoot(id string) error {
	return c.doNoContent("DELETE", fmt.Sprintf("/app/rest/vcs-roots/id:%s", url.PathEscape(id)), nil, "")
}

func (c *Client) VcsRootExists(id string) bool {
	_, err := c.GetVcsRoot(id)
	return err == nil
}

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

	return c.doNoContent("POST", fmt.Sprintf("/app/rest/buildTypes/id:%s/vcs-root-entries", buildTypeID), bytes.NewReader(body), "")
}

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

func NewGitVcsRootProperties(repoURL, branch string) PropertyList {
	props := []Property{
		{Name: "url", Value: repoURL},
	}
	if branch != "" {
		props = append(props, Property{Name: "branch", Value: branch})
	}
	return PropertyList{Property: props}
}
