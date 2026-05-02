package api

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// PinBuild pins a build to prevent it from being cleaned up (accepts ID or #number)
func (c *Client) PinBuild(buildID string, comment string) error {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/pin", id)

	body := cmp.Or(comment, "Pinned via teamcity CLI")

	return c.doNoContent(c.ctx(), "PUT", path, strings.NewReader(body), "text/plain")
}

// UnpinBuild removes the pin from a build (accepts ID or #number)
func (c *Client) UnpinBuild(buildID string) error {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/pin", id)
	return c.doNoContent(c.ctx(), "DELETE", path, nil, "")
}

// AddBuildTags adds tags to a build (accepts ID or #number)
func (c *Client) AddBuildTags(buildID string, tags []string) error {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/tags", id)

	tagList := TagList{Tag: make([]Tag, len(tags))}
	for i, t := range tags {
		tagList.Tag[i] = Tag{Name: t}
	}

	body, err := json.Marshal(tagList)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	resp, err := c.doRequest(c.ctx(), "POST", path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// GetBuildTags returns the tags for a build (accepts ID or #number)
func (c *Client) GetBuildTags(buildID string) (*TagList, error) {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/tags", id)

	var tags TagList
	if err := c.get(c.ctx(), path, &tags); err != nil {
		return nil, err
	}

	return &tags, nil
}

// RemoveBuildTag removes a specific tag from a build (accepts ID or #number)
func (c *Client) RemoveBuildTag(buildID string, tag string) error {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return err
	}

	currentTags, err := c.GetBuildTags(id)
	if err != nil {
		return fmt.Errorf("failed to get current tags: %w", err)
	}

	var newTags []Tag
	found := false
	for _, t := range currentTags.Tag {
		if t.Name != tag {
			newTags = append(newTags, t)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("tag '%s' not found on build", tag)
	}

	path := fmt.Sprintf("/app/rest/builds/id:%s/tags", id)
	tagList := TagList{Tag: newTags}

	body, err := json.Marshal(tagList)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	return c.doNoContent(c.ctx(), "PUT", path, bytes.NewReader(body), "")
}

// SetBuildComment sets or updates the comment on a build (accepts ID or #number)
func (c *Client) SetBuildComment(buildID string, comment string) error {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/comment", id)
	return c.doNoContent(c.ctx(), "PUT", path, strings.NewReader(comment), "text/plain")
}

// buildWithComment is used to fetch just the comment from a build
type buildWithComment struct {
	Comment *BuildComment `json:"comment,omitempty"`
}

// GetBuildComment returns the comment for a build (accepts ID or #number)
func (c *Client) GetBuildComment(buildID string) (string, error) {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s?fields=comment(text)", id)

	var result buildWithComment
	if err := c.get(c.ctx(), path, &result); err != nil {
		return "", err
	}

	if result.Comment == nil {
		return "", nil // No comment set
	}

	return result.Comment.Text, nil
}

// DeleteBuildComment removes the comment from a build (accepts ID or #number)
func (c *Client) DeleteBuildComment(buildID string) error {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/comment", id)
	return c.doNoContent(c.ctx(), "DELETE", path, nil, "")
}

func (c *Client) GetBuildChanges(ctx context.Context, buildID string) (*ChangeList, error) {
	id, err := c.ResolveBuildID(ctx, buildID)
	if err != nil {
		return nil, err
	}

	fields := "count,change(id,version,username,date,comment,files(file(file,changeType)))"
	path := fmt.Sprintf("/app/rest/changes?locator=build:(id:%s)&fields=%s", id, url.QueryEscape(fields))

	var changes ChangeList
	if err := c.get(ctx, path, &changes); err != nil {
		return nil, err
	}

	return &changes, nil
}

func (c *Client) UploadDiffChanges(patch []byte, description string) (string, error) {
	uploadURL := fmt.Sprintf("/uploadDiffChanges.html?description=%s&commitType=0",
		url.QueryEscape(description))

	resp, err := c.RawRequest(c.ctx(), "POST", uploadURL, bytes.NewReader(patch), map[string]string{
		"Content-Type": "text/plain",
		"Origin":       c.BaseURL,
	})
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", errorFromBody(resp.StatusCode, resp.Body)
	}

	return strings.TrimSpace(string(resp.Body)), nil
}

// BuildTestsOptions controls test occurrence filtering for a build.
type BuildTestsOptions struct {
	FailedOnly bool
	MutedOnly  bool
	Limit      int
}

func (c *Client) GetBuildTests(ctx context.Context, buildID string, opts BuildTestsOptions) (*TestOccurrences, error) {
	if opts.FailedOnly && opts.MutedOnly {
		return nil, Validation("failedOnly and mutedOnly are mutually exclusive", "set only one test result filter")
	}

	id, err := c.ResolveBuildID(ctx, buildID)
	if err != nil {
		return nil, err
	}

	locator := NewLocator().AddLocator("build", NewLocator().Add("id", id))
	switch {
	case opts.FailedOnly:
		locator.AddUpper("status", "FAILURE").Add("muted", "false")
	case opts.MutedOnly:
		locator.AddUpper("status", "FAILURE").Add("muted", "true")
	}

	summaryFields := "count,passed,failed,ignored,muted"
	summaryPath := fmt.Sprintf("/app/rest/testOccurrences?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(summaryFields))

	var summary TestOccurrences
	if err := c.get(ctx, summaryPath, &summary); err != nil {
		return nil, err
	}

	count := opts.Limit
	if count <= 0 {
		count = summary.Count
	}
	locator.AddInt("count", count)

	detailFields := "testOccurrence(id,name,status,duration,details,newFailure,muted,firstFailed(build(id,number)))"
	detailPath := fmt.Sprintf("/app/rest/testOccurrences?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(detailFields))

	var details TestOccurrences
	if err := c.get(ctx, detailPath, &details); err != nil {
		return nil, err
	}

	summary.TestOccurrence = details.TestOccurrence
	return &summary, nil
}

func (c *Client) GetBuildTestSummary(buildID string) (*TestOccurrences, error) {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return nil, err
	}

	locator := fmt.Sprintf("build:(id:%s)", id)
	fields := "count,passed,failed,ignored,muted"
	path := fmt.Sprintf("/app/rest/testOccurrences?locator=%s&fields=%s", url.QueryEscape(locator), url.QueryEscape(fields))

	var summary TestOccurrences
	if err := c.get(c.ctx(), path, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func (c *Client) GetBuildResultingProperties(buildID string) (*ParameterList, error) {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/app/rest/builds/id:%s/resulting-properties", id)

	var params ParameterList
	if err := c.get(c.ctx(), path, &params); err != nil {
		return nil, err
	}

	return &params, nil
}

func (c *Client) GetBuildProblems(buildID string) (*ProblemOccurrences, error) {
	id, err := c.ResolveBuildID(c.ctx(), buildID)
	if err != nil {
		return nil, err
	}

	locator := fmt.Sprintf("build:(id:%s)", id)
	fields := "count,problemOccurrence(id,type,identity,details)"
	path := fmt.Sprintf("/app/rest/problemOccurrences?locator=%s&fields=%s", url.QueryEscape(locator), url.QueryEscape(fields))

	var problems ProblemOccurrences
	if err := c.get(c.ctx(), path, &problems); err != nil {
		return nil, err
	}

	return &problems, nil
}
