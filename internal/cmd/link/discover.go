package link

import (
	"cmp"
	"slices"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/git"
)

// projectMatch groups all build types in a single TC project that match a repo's remotes.
type projectMatch struct {
	ProjectID   string
	ProjectName string
	Jobs        []jobOption
}

// jobOption is one selectable job/pipeline-head; Label is what the picker renders.
type jobOption struct {
	ID          string
	Name        string
	ProjectName string
	Label       string
	Pipeline    bool
}

// discovery is the per-server input bundle the picker form consumes.
type discovery struct {
	Projects []projectMatch
}

// discoveryBuildTypeFields is dot-notation expanded by ToAPIFields into the nested REST form.
var discoveryBuildTypeFields = []string{
	"id", "name", "projectId", "projectName", "paused",
	"vcs-root-entries.vcs-root-entry.vcs-root.properties.property.name",
	"vcs-root-entries.vcs-root-entry.vcs-root.properties.property.value",
}

// extractFragments returns server-side substring fragments and a canonical set used for fork rejection.
func extractFragments(remoteURLs []string) (fragments, canonical []string) {
	seenFrag := map[string]bool{}
	seenCanon := map[string]bool{}
	for _, raw := range remoteURLs {
		canon := git.CanonicalURL(raw)
		repoPath := git.RepoPath(raw)
		if canon != "" && !seenCanon[canon] {
			seenCanon[canon] = true
			canonical = append(canonical, canon)
		}
		// Prefer the org/repo form as fragment — matches across hosts (github.com/x/y, gitlab.x/x/y).
		if repoPath != "" && !seenFrag[repoPath] {
			seenFrag[repoPath] = true
			fragments = append(fragments, repoPath)
		}
	}
	return fragments, canonical
}

// buildTypeMatchesRemotes reports whether bt's VCS roots include a URL whose canonical form is in the user's set.
func buildTypeMatchesRemotes(bt api.BuildType, canonicalRemotes []string) bool {
	if bt.VcsRootEntries == nil {
		return false
	}
	for _, entry := range bt.VcsRootEntries.VcsRootEntry {
		if entry.VcsRoot == nil || entry.VcsRoot.Properties == nil {
			continue
		}
		for _, p := range entry.VcsRoot.Properties.Property {
			if p.Name != "url" {
				continue
			}
			if canon := git.CanonicalURL(p.Value); canon != "" && slices.Contains(canonicalRemotes, canon) {
				return true
			}
		}
	}
	return false
}

// pipelineMeta carries the parent project so a pipeline-head buildType binds to the parent, not TC's auto-created wrapper.
type pipelineMeta struct {
	Name       string
	ParentID   string
	ParentName string
}

// discoverProjects fetches build types whose VCS roots match remoteURLs and groups them by project; nil result means no remotes or no matches.
func discoverProjects(client api.ClientInterface, remoteURLs []string) (*discovery, error) {
	fragments, canonical := extractFragments(remoteURLs)
	if len(fragments) == 0 {
		return nil, nil
	}

	pipelineMetaByHead := map[string]pipelineMeta{}
	if client.SupportsFeature("pipelines") {
		pipelines, err := client.GetPipelines(api.PipelinesOptions{Limit: 0})
		if err == nil && pipelines != nil {
			for _, p := range pipelines.Pipelines {
				if p.HeadBuildType == nil || p.HeadBuildType.ID == "" {
					continue
				}
				m := pipelineMeta{Name: p.Name}
				if p.ParentProject != nil {
					m.ParentID = p.ParentProject.ID
					m.ParentName = p.ParentProject.Name
				}
				pipelineMetaByHead[p.HeadBuildType.ID] = m
			}
		}
	}

	seenBuildTypes := map[string]bool{}
	var matched []api.BuildType
	for _, frag := range fragments {
		page, err := client.GetBuildTypes(api.BuildTypesOptions{
			VcsRootURL: frag,
			Limit:      0,
			Fields:     discoveryBuildTypeFields,
		})
		if err != nil {
			return nil, err
		}
		for _, bt := range page.BuildTypes {
			if bt.Paused {
				continue
			}
			if seenBuildTypes[bt.ID] {
				continue
			}
			if !buildTypeMatchesRemotes(bt, canonical) {
				continue
			}
			seenBuildTypes[bt.ID] = true
			matched = append(matched, bt)
		}
	}

	if len(matched) == 0 {
		return nil, nil
	}

	byProject := map[string]*projectMatch{}
	for _, bt := range matched {
		projID, projName := bt.ProjectID, bt.ProjectName
		opt := jobOption{ID: bt.ID, Name: bt.Name}
		if meta, isHead := pipelineMetaByHead[bt.ID]; isHead {
			opt.Pipeline = true
			opt.Name = meta.Name
			if meta.ParentID != "" {
				projID, projName = meta.ParentID, meta.ParentName
			}
		}
		opt.ProjectName = projName
		opt.Label = jobLabel(opt)

		pm, ok := byProject[projID]
		if !ok {
			pm = &projectMatch{ProjectID: projID, ProjectName: projName}
			byProject[projID] = pm
		}
		pm.Jobs = append(pm.Jobs, opt)
	}

	d := &discovery{}
	for _, pm := range byProject {
		slices.SortFunc(pm.Jobs, func(a, b jobOption) int { return cmp.Compare(a.Name, b.Name) })
		d.Projects = append(d.Projects, *pm)
	}
	slices.SortFunc(d.Projects, func(a, b projectMatch) int { return cmp.Compare(a.ProjectName, b.ProjectName) })
	return d, nil
}

// jobLabel renders the picker label for a job, with a "⬡ pipeline" marker for pipeline heads.
func jobLabel(o jobOption) string {
	prefix := o.ProjectName
	if prefix != "" {
		prefix += " · "
	}
	if o.Pipeline {
		return prefix + o.Name + "  ⬡ pipeline"
	}
	return prefix + o.Name
}
