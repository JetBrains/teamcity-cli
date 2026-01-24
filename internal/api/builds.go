package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// BuildsOptions represents options for listing builds
type BuildsOptions struct {
	BuildTypeID string
	Branch      string
	Status      string
	State       string
	User        string
	Project     string
	Number      string
	Limit       int
	SinceDate   string
	UntilDate   string
	Fields      []string
}

// GetBuilds returns a list of builds
func (c *Client) GetBuilds(opts BuildsOptions) (*BuildList, error) {
	locator := NewLocator().
		Add("buildType", opts.BuildTypeID).
		Add("branch", opts.Branch).
		AddUpper("status", opts.Status).
		Add("state", opts.State).
		Add("user", opts.User).
		Add("affectedProject", opts.Project).
		Add("number", opts.Number).
		Add("sinceDate", opts.SinceDate).
		Add("untilDate", opts.UntilDate).
		AddIntDefault("count", opts.Limit, 30)

	buildFields := opts.Fields
	if len(buildFields) == 0 {
		buildFields = BuildFields.Default
	}
	fields := fmt.Sprintf("count,build(%s)", ToAPIFields(buildFields))
	path := fmt.Sprintf("/app/rest/builds?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fields))

	var result BuildList
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	for i := range result.Builds {
		cleanupBuildTriggered(&result.Builds[i])
	}

	return &result, nil
}

// cleanupBuildTriggered removes empty User objects from build trigger info
func cleanupBuildTriggered(b *Build) {
	if b.Triggered != nil && b.Triggered.User != nil {
		u := b.Triggered.User
		if u.ID == 0 && u.Username == "" && u.Name == "" && u.Email == "" {
			b.Triggered.User = nil
		}
	}
}

// ResolveBuildID resolves a build reference to an ID.
// If ref starts with #, it's treated as a build number and looked up.
// Otherwise it's used as-is (assumed to be an ID).
func (c *Client) ResolveBuildID(ref string) (string, error) {
	if !strings.HasPrefix(ref, "#") {
		return ref, nil
	}

	number := strings.TrimPrefix(ref, "#")
	builds, err := c.GetBuilds(BuildsOptions{Limit: 1, Number: number})
	if err != nil {
		return "", err
	}
	if builds.Count == 0 {
		return "", fmt.Errorf("no build found with number %s", ref)
	}
	return fmt.Sprintf("%d", builds.Builds[0].ID), nil
}

// GetBuild returns a single build by ID or #number
func (c *Client) GetBuild(ref string) (*Build, error) {
	id, err := c.ResolveBuildID(ref)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/app/rest/builds/id:%s", id)

	var build Build
	if err := c.get(path, &build); err != nil {
		return nil, err
	}

	return &build, nil
}

// RunBuildOptions represents options for running a build
type RunBuildOptions struct {
	Branch                    string
	Params                    map[string]string // Configuration parameters
	SystemProps               map[string]string // System properties (system.*)
	EnvVars                   map[string]string // Environment variables (env.*)
	Comment                   string
	Personal                  bool
	CleanSources              bool
	RebuildDependencies       bool
	QueueAtTop                bool
	RebuildFailedDependencies bool
	AgentID                   int
	Tags                      []string
}

// RunBuild runs a new build with full options
func (c *Client) RunBuild(buildTypeID string, opts RunBuildOptions) (*Build, error) {
	req := TriggerBuildRequest{
		BuildType: BuildTypeRef{ID: buildTypeID},
	}

	if opts.Branch != "" {
		req.BranchName = opts.Branch
	}

	var props []Property
	for k, v := range opts.Params {
		props = append(props, Property{Name: k, Value: v})
	}
	for k, v := range opts.SystemProps {
		props = append(props, Property{Name: "system." + k, Value: v})
	}
	for k, v := range opts.EnvVars {
		props = append(props, Property{Name: "env." + k, Value: v})
	}
	if len(props) > 0 {
		req.Properties = &PropertyList{Property: props}
	}

	if opts.Comment != "" {
		req.Comment = &BuildComment{Text: opts.Comment}
	}

	req.Personal = opts.Personal

	if opts.CleanSources || opts.RebuildDependencies || opts.QueueAtTop || opts.RebuildFailedDependencies {
		req.TriggeringOptions = &TriggeringOptions{
			CleanSources:              opts.CleanSources,
			RebuildAllDependencies:    opts.RebuildDependencies,
			QueueAtTop:                opts.QueueAtTop,
			RebuildFailedOrIncomplete: opts.RebuildFailedDependencies,
		}
	}

	if opts.AgentID > 0 {
		req.Agent = &AgentRef{ID: opts.AgentID}
	}

	if len(opts.Tags) > 0 {
		var tags []Tag
		for _, t := range opts.Tags {
			tags = append(tags, Tag{Name: t})
		}
		req.Tags = &TagList{Tag: tags}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var build Build
	if err := c.post("/app/rest/buildQueue", bytes.NewReader(body), &build); err != nil {
		return nil, err
	}

	return &build, nil
}

// CancelBuild cancels a running or queued build (accepts ID or #number)
func (c *Client) CancelBuild(buildID string, comment string) error {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/cancel", id)

	body := struct {
		Comment        string `json:"comment"`
		ReaddIntoQueue bool   `json:"readdIntoQueue"`
	}{
		Comment:        comment,
		ReaddIntoQueue: false,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	return c.doNoContent("POST", path, bytes.NewReader(bodyBytes), "")
}

// QueueOptions represents options for listing queued builds
type QueueOptions struct {
	BuildTypeID string
	Limit       int
	Fields      []string
}

// GetBuildQueue returns the build queue
func (c *Client) GetBuildQueue(opts QueueOptions) (*BuildQueue, error) {
	locator := NewLocator().
		Add("buildType", opts.BuildTypeID).
		AddInt("count", opts.Limit)

	fields := opts.Fields
	if len(fields) == 0 {
		fields = QueuedBuildFields.Default
	}
	fieldsParam := fmt.Sprintf("count,build(%s)", ToAPIFields(fields))

	path := "/app/rest/buildQueue"
	if !locator.IsEmpty() {
		path = fmt.Sprintf("%s?locator=%s&fields=%s", path, locator.Encode(), url.QueryEscape(fieldsParam))
	} else {
		path = fmt.Sprintf("%s?fields=%s", path, url.QueryEscape(fieldsParam))
	}

	var queue BuildQueue
	if err := c.get(path, &queue); err != nil {
		return nil, err
	}
	return &queue, nil
}

// RemoveFromQueue removes a build from the queue
func (c *Client) RemoveFromQueue(id string) error {
	path := fmt.Sprintf("/app/rest/buildQueue/id:%s", id)
	return c.doNoContent("DELETE", path, nil, "")
}

// Artifact represents a build artifact
type Artifact struct {
	Name     string     `json:"name"`
	Size     int64      `json:"size,omitempty"`
	ModTime  string     `json:"modificationTime,omitempty"`
	Href     string     `json:"href,omitempty"`
	Children *Artifacts `json:"children,omitempty"`
	Content  *Content   `json:"content,omitempty"`
}

// Content represents artifact content reference
type Content struct {
	Href string `json:"href"`
}

// Artifacts represents a list of artifacts
type Artifacts struct {
	Count int        `json:"count"`
	File  []Artifact `json:"file"`
}

// GetArtifacts returns the artifacts for a build (accepts ID or #number)
func (c *Client) GetArtifacts(buildID string) (*Artifacts, error) {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/artifacts/children", id)

	var artifacts Artifacts
	if err := c.get(path, &artifacts); err != nil {
		return nil, err
	}

	return &artifacts, nil
}

// DownloadArtifact downloads an artifact and returns its content (accepts ID or #number)
func (c *Client) DownloadArtifact(buildID, artifactPath string) ([]byte, error) {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/artifacts/content/%s", id, artifactPath)

	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to download artifact: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// GetBuildLog returns the build log (accepts ID or #number)
func (c *Client) GetBuildLog(buildID string) (string, error) {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/downloadBuildLog.html?buildId=%s", id)

	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get build log: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// PinBuild pins a build to prevent it from being cleaned up (accepts ID or #number)
func (c *Client) PinBuild(buildID string, comment string) error {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/pin", id)

	body := comment
	if body == "" {
		body = "Pinned via tc CLI"
	}

	return c.doNoContent("PUT", path, strings.NewReader(body), "text/plain")
}

// UnpinBuild removes the pin from a build (accepts ID or #number)
func (c *Client) UnpinBuild(buildID string) error {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/pin", id)
	return c.doNoContent("DELETE", path, nil, "")
}

// AddBuildTags adds tags to a build (accepts ID or #number)
func (c *Client) AddBuildTags(buildID string, tags []string) error {
	id, err := c.ResolveBuildID(buildID)
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

	resp, err := c.doRequest("POST", path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// GetBuildTags returns the tags for a build (accepts ID or #number)
func (c *Client) GetBuildTags(buildID string) (*TagList, error) {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/tags", id)

	var tags TagList
	if err := c.get(path, &tags); err != nil {
		return nil, err
	}

	return &tags, nil
}

// RemoveBuildTag removes a specific tag from a build (accepts ID or #number)
func (c *Client) RemoveBuildTag(buildID string, tag string) error {
	id, err := c.ResolveBuildID(buildID)
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

	return c.doNoContent("PUT", path, bytes.NewReader(body), "")
}

// SetBuildComment sets or updates the comment on a build (accepts ID or #number)
func (c *Client) SetBuildComment(buildID string, comment string) error {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/comment", id)
	return c.doNoContent("PUT", path, strings.NewReader(comment), "text/plain")
}

// buildWithComment is used to fetch just the comment from a build
type buildWithComment struct {
	Comment *BuildComment `json:"comment,omitempty"`
}

// GetBuildComment returns the comment for a build (accepts ID or #number)
func (c *Client) GetBuildComment(buildID string) (string, error) {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s?fields=comment(text)", id)

	var result buildWithComment
	if err := c.get(path, &result); err != nil {
		return "", err
	}

	if result.Comment == nil {
		return "", nil // No comment set
	}

	return result.Comment.Text, nil
}

// DeleteBuildComment removes the comment from a build (accepts ID or #number)
func (c *Client) DeleteBuildComment(buildID string) error {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/app/rest/builds/id:%s/comment", id)
	return c.doNoContent("DELETE", path, nil, "")
}

// SetQueuedBuildPosition moves a queued build to a specific position in the queue
func (c *Client) SetQueuedBuildPosition(buildID string, position int) error {
	path := fmt.Sprintf("/app/rest/buildQueue/order/%s", buildID)
	return c.doNoContent("PUT", path, strings.NewReader(fmt.Sprintf("%d", position)), "text/plain")
}

// MoveQueuedBuildToTop moves a queued build to the top of the queue
func (c *Client) MoveQueuedBuildToTop(buildID string) error {
	return c.SetQueuedBuildPosition(buildID, 0)
}

// ApproveQueuedBuild approves a queued build that requires approval
func (c *Client) ApproveQueuedBuild(buildID string) error {
	path := fmt.Sprintf("/app/rest/buildQueue/id:%s/approval/status", buildID)
	return c.doNoContent("PUT", path, strings.NewReader(`"approved"`), "application/json")
}

// GetQueuedBuildApprovalInfo returns approval information for a queued build
func (c *Client) GetQueuedBuildApprovalInfo(buildID string) (*ApprovalInfo, error) {
	path := fmt.Sprintf("/app/rest/buildQueue/id:%s/approval", buildID)

	var info ApprovalInfo
	if err := c.get(path, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

func (c *Client) GetBuildChanges(buildID string) (*ChangeList, error) {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return nil, err
	}

	fields := "count,change(id,version,username,date,comment,files(file(file,changeType)))"
	path := fmt.Sprintf("/app/rest/changes?locator=build:(id:%s)&fields=%s", id, url.QueryEscape(fields))

	var changes ChangeList
	if err := c.get(path, &changes); err != nil {
		return nil, err
	}

	return &changes, nil
}

func (c *Client) GetBuildTests(buildID string, failedOnly bool, limit int) (*TestOccurrences, error) {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return nil, err
	}

	baseLocator := fmt.Sprintf("build:(id:%s)", id)
	if failedOnly {
		baseLocator += ",status:FAILURE"
	}

	summaryFields := "count,passed,failed,ignored"
	summaryPath := fmt.Sprintf("/app/rest/testOccurrences?locator=%s&fields=%s", url.QueryEscape(baseLocator), url.QueryEscape(summaryFields))

	var summary TestOccurrences
	if err := c.get(summaryPath, &summary); err != nil {
		return nil, err
	}

	detailLocator := baseLocator
	if limit > 0 {
		detailLocator += fmt.Sprintf(",count:%d", limit)
	} else {
		detailLocator += fmt.Sprintf(",count:%d", summary.Count)
	}

	detailFields := "testOccurrence(id,name,status,duration)"
	detailPath := fmt.Sprintf("/app/rest/testOccurrences?locator=%s&fields=%s", url.QueryEscape(detailLocator), url.QueryEscape(detailFields))

	var details TestOccurrences
	if err := c.get(detailPath, &details); err != nil {
		return nil, err
	}

	summary.TestOccurrence = details.TestOccurrence
	return &summary, nil
}
