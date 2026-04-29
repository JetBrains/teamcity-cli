package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// GetPipelines lists pipelines, optionally filtered by project. Automatically follows pagination.
func (c *Client) GetPipelines(opts PipelinesOptions) (*PipelineList, error) {
	locator := NewLocator().
		AddIntDefault("count", opts.Limit, 100)
	if opts.Project != "" {
		locator.AddLocator("parentProject", NewLocator().Add("id", opts.Project))
	}

	fields := opts.Fields
	if len(fields) == 0 {
		fields = PipelineFields.Default
	}
	fieldsParam := fmt.Sprintf("count,nextHref,pipeline(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/pipelines?locator=%s&fields=%s",
		locator.Encode(), url.QueryEscape(fieldsParam))

	pipelines, err := collectPages(c, path, opts.Limit, func(p string) ([]Pipeline, string, error) {
		var page PipelineList
		if err := c.get(c.ctx(), p, &page); err != nil {
			return nil, "", err
		}
		return page.Pipelines, page.NextHref, nil
	})
	if err != nil {
		return nil, err
	}
	return &PipelineList{Count: len(pipelines), Pipelines: pipelines}, nil
}

// GetPipeline retrieves a single pipeline by ID via the REST API.
func (c *Client) GetPipeline(id string) (*Pipeline, error) {
	fields := "id,name,webUrl,parentProject(id,name),headBuildType(id),jobs(count,job(id,name))"
	path := fmt.Sprintf("/app/rest/pipelines/id:%s?fields=%s", id, url.QueryEscape(fields))

	var result Pipeline
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// getPipelineRaw fetches the full pipeline state from /app/pipeline/{id}.
// This non-REST endpoint returns YAML, VCS root details, triggers, and other settings.
func (c *Client) getPipelineRaw(id string) (map[string]any, error) {
	path := "/app/pipeline/" + id
	var raw map[string]any
	if err := c.get(c.ctx(), path, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// GetPipelineYAML retrieves the YAML source of a pipeline.
// Returns empty string if the pipeline stores YAML in VCS.
func (c *Client) GetPipelineYAML(id string) (string, error) {
	raw, err := c.getPipelineRaw(id)
	if err != nil {
		return "", err
	}
	if vs, ok := raw["versionedSettings"].(map[string]any); ok {
		if stored, ok := vs["storedInRepo"].(bool); ok && stored {
			return "", nil
		}
	}
	yaml, _ := raw["yaml"].(string)
	return yaml, nil
}

// CreatePipeline creates a new pipeline in the given project with a VCS root.
func (c *Client) CreatePipeline(parentProjectID, name, yaml, vcsRootID string) (*Pipeline, error) {
	if c.ReadOnly {
		return nil, fmt.Errorf("%w: POST /app/pipeline", ErrReadOnly)
	}

	req := CreatePipelineRequest{
		Name:    name,
		YAML:    yaml,
		VcsRoot: &PipelineVcsRootRef{ExternalVcsRootID: vcsRootID},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	path := "/app/pipeline?parentProjectExtId=" + url.QueryEscape(parentProjectID)
	var result Pipeline
	if err := c.post(c.ctx(), path, bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdatePipelineYAML replaces the YAML content of an existing pipeline.
// Preserves all other pipeline settings (VCS root, triggers, notifications).
func (c *Client) UpdatePipelineYAML(id string, yamlContent string) error {
	raw, err := c.getPipelineRaw(id)
	if err != nil {
		return fmt.Errorf("failed to get pipeline: %w", err)
	}

	update := map[string]any{
		"name": raw["name"],
		"yaml": yamlContent,
	}
	if vcsRoot, ok := raw["vcsRoot"].(map[string]any); ok {
		if vcsID, ok := vcsRoot["id"].(string); ok {
			update["vcsRoot"] = map[string]any{"externalVcsRootId": vcsID}
		}
	}
	for _, key := range []string{"additionalVcsRoots", "triggers", "integrations", "notifications"} {
		if v, ok := raw[key]; ok {
			update[key] = v
		}
	}

	body, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	path := "/app/pipeline/" + id
	var result json.RawMessage
	return c.post(c.ctx(), path, bytes.NewReader(body), &result)
}

// GetBuildPipelineRun fetches pipeline run metadata for a build.
// Returns nil if the build is not a pipeline run.
func (c *Client) GetBuildPipelineRun(buildID string) (*PipelineRun, error) {
	fields := "pipelineRun(number,pipeline(id,name),jobs(count,job(id,name,build(id))))"
	path := fmt.Sprintf("/app/rest/builds/id:%s?fields=%s", buildID, url.QueryEscape(fields))

	var result struct {
		PipelineRun *PipelineRun `json:"pipelineRun,omitempty"`
	}
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}
	return result.PipelineRun, nil
}

// DeletePipeline deletes a pipeline by removing its project.
func (c *Client) DeletePipeline(id string) error {
	return c.doNoContent(c.ctx(), "DELETE", "/app/rest/projects/id:"+id, nil, "")
}

// ErrPipelineSchemaUnsupported is returned when the server's schema endpoint exists but does not produce JSON, indicating a TeamCity version older than 2026.1.
var ErrPipelineSchemaUnsupported = errors.New("schema endpoint not available on this server")

// GetPipelineSchema fetches the pipeline JSON schema from the server.
func (c *Client) GetPipelineSchema() ([]byte, error) {
	resp, err := c.doRequest(c.ctx(), "POST", "/app/pipeline/schema/generate", nil)
	if err != nil {
		return nil, &NetworkError{URL: c.BaseURL, Cause: err}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		return nil, ErrPipelineSchemaUnsupported
	}

	return io.ReadAll(resp.Body)
}
