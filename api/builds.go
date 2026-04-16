package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
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
	Revision    string
	Favorites   bool
	Limit       int
	SinceDate   string
	UntilDate   string
	Fields      []string
}

const favoriteBuildTag = ".teamcity.star"

// Locator builds the TeamCity locator used to fetch builds.
func (opts BuildsOptions) Locator() *Locator {
	locator := NewLocator().
		Add("buildType", opts.BuildTypeID).
		Add("defaultFilter", "false")
	if opts.Branch != "" {
		locator.Add("branch", opts.Branch)
	} else {
		locator.AddLocator("branch", NewLocator().Add("default", "any"))
	}
	locator.
		AddUpper("status", opts.Status).
		Add("state", opts.State).
		Add("user", opts.User).
		Add("affectedProject", opts.Project).
		Add("number", opts.Number).
		Add("revision", opts.Revision).
		Add("sinceDate", opts.SinceDate).
		Add("untilDate", opts.UntilDate)
	if opts.Favorites {
		locator.AddLocator("tag", currentUserFavoriteBuildsTagLocator())
	}
	return locator
}

func currentUserFavoriteBuildsTagLocator() *Locator {
	return NewLocator().
		Add("private", "true").
		Add("owner", "current").
		AddLocator("condition", NewLocator().
			Add("value", favoriteBuildTag).
			Add("matchType", "equals").
			Add("ignoreCase", "false"))
}

// GetBuilds returns a list of builds
func (c *Client) GetBuilds(opts BuildsOptions) (*BuildList, error) {
	locator := opts.Locator().
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

// GetBuildUsedByOtherBuilds checks whether a build's results were shared with other builds.
// This field is not included in TC's default response, so it requires a targeted request.
func (c *Client) GetBuildUsedByOtherBuilds(id string) (bool, error) {
	path := fmt.Sprintf("/app/rest/builds/id:%s?fields=usedByOtherBuilds", id)
	var result struct {
		UsedByOtherBuilds bool `json:"usedByOtherBuilds"`
	}
	if err := c.get(path, &result); err != nil {
		return false, err
	}
	return result.UsedByOtherBuilds, nil
}

// buildState is a lightweight struct for polling build status with minimal fields.
type buildState struct {
	State              string `json:"state"`
	Status             string `json:"status"`
	PercentageComplete int    `json:"percentageComplete"`
}

// WaitForBuildOptions configures the WaitForBuild polling behavior.
type WaitForBuildOptions struct {
	Interval time.Duration
	// OnProgress is called after each poll with the current state.
	// Return a non-nil error to abort the wait.
	OnProgress func(state, status string, percent int) error
}

// WaitForBuild polls a build until it reaches state "finished", then returns the full build.
// Uses lightweight field-limited requests for polling, and fetches the complete build only once.
func (c *Client) WaitForBuild(ctx context.Context, buildID string, opts WaitForBuildOptions) (*Build, error) {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return nil, err
	}

	interval := opts.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}

	pollPath := fmt.Sprintf("/app/rest/builds/id:%s?fields=state,status,percentageComplete", id)

	for {
		var bs buildState
		if err := c.get(pollPath, &bs); err != nil {
			return nil, err
		}

		if opts.OnProgress != nil {
			if err := opts.OnProgress(bs.State, bs.Status, bs.PercentageComplete); err != nil {
				return nil, err
			}
		}

		if bs.State == "finished" {
			return c.getFinishedBuild(ctx, id)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// getFinishedBuild fetches the full build after state transitions to "finished".
// TeamCity briefly reports status as "UNKNOWN" during post-processing; this retries
// a few times to let the final status (SUCCESS/FAILURE/etc.) settle.
func (c *Client) getFinishedBuild(ctx context.Context, id string) (*Build, error) {
	for range 10 {
		build, err := c.GetBuild(id)
		if err != nil {
			return nil, err
		}
		if build.Status != "UNKNOWN" {
			return build, nil
		}
		select {
		case <-ctx.Done():
			return build, nil // return what we have rather than a bare context error
		case <-time.After(500 * time.Millisecond):
		}
	}
	return c.GetBuild(id) // final attempt
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
	PersonalChangeID          string
	Revision                  string
	SnapshotDependencies      []int
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

	if opts.PersonalChangeID != "" {
		req.LastChanges = &LastChanges{
			Change: []PersonalChange{
				{ID: opts.PersonalChangeID, Personal: true},
			},
		}
	}

	if len(opts.SnapshotDependencies) > 0 {
		refs := make([]BuildRef, len(opts.SnapshotDependencies))
		for i, id := range opts.SnapshotDependencies {
			refs[i] = BuildRef{ID: id}
		}
		req.SnapshotDependencies = &SnapshotDepBuilds{Build: refs}
	}

	if opts.Revision != "" {
		entries, err := c.GetVcsRootEntries(buildTypeID)
		if err != nil {
			return nil, fmt.Errorf("failed to get VCS root entries: %w", err)
		}
		if entries.Count == 0 {
			return nil, fmt.Errorf("build configuration %s has no VCS roots; cannot pin revision", buildTypeID)
		}

		branch := opts.Branch
		if branch != "" && !strings.HasPrefix(branch, "refs/") {
			branch = "refs/heads/" + branch
		}

		var revisions []Revision
		for _, entry := range entries.VcsRootEntry {
			vcsRootID := ""
			if entry.VcsRoot != nil {
				vcsRootID = entry.VcsRoot.ID
			}
			if vcsRootID == "" {
				continue
			}
			rev := Revision{
				Version:         opts.Revision,
				VcsBranchName:   branch,
				VcsRootInstance: &VcsRootInstanceRef{VcsRootID: vcsRootID},
			}
			revisions = append(revisions, rev)
		}
		if len(revisions) > 0 {
			req.Revisions = &Revisions{Revision: revisions}
		}
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

	build, err := c.GetBuild(id)
	if err != nil {
		return err
	}

	if build.State == "finished" {
		return fmt.Errorf("cannot cancel finished build")
	}

	if build.State == "queued" {
		return c.RemoveFromQueue(id)
	}

	path := fmt.Sprintf("/app/rest/builds/id:%s", id)

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

// GetBuildSnapshotDependencies returns all immediate dependency builds in a snapshot dependency chain.
func (c *Client) GetBuildSnapshotDependencies(buildID string) (*BuildList, error) {
	locator := fmt.Sprintf("snapshotDependency:(to:(id:%s),recursive:false),defaultFilter:false", buildID)
	fields := "count,nextHref,build(id,number,status,statusText,state,buildTypeId,buildType(id,name))"
	path := fmt.Sprintf("/app/rest/builds?locator=%s&fields=%s", url.QueryEscape(locator), url.QueryEscape(fields))

	var combined BuildList
	for path != "" {
		var page BuildList
		if err := c.get(path, &page); err != nil {
			return nil, err
		}
		combined.Builds = append(combined.Builds, page.Builds...)
		combined.Count += page.Count
		path = page.NextHref
		if next, err := url.Parse(path); err == nil && next.IsAbs() {
			path = next.RequestURI()
		}
		if base, err := url.Parse(c.BaseURL); err == nil && len(base.Path) > 1 {
			path = strings.TrimPrefix(path, base.Path)
		}
		if c.APIVersion != "" && strings.HasPrefix(path, "/app/rest/") {
			path = strings.Replace(path, "/app/rest/"+c.APIVersion+"/", "/app/rest/", 1)
		}
	}
	return &combined, nil
}
