package link

import (
	"sort"
	"strings"
	"sync"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/git"
)

// projectMatch is a TeamCity project surfaced by discovery; its Jobs are the build configs that reference the repo's VCS URL.
type projectMatch struct {
	ID   string
	Name string
	Jobs []api.BuildType
}

// discovery bundles the picker inputs: projects, the flat job list across them, and a pipelineID→name lookup for label enrichment.
type discovery struct {
	Projects  []projectMatch
	AllJobs   []api.BuildType
	Pipelines map[string]string
}

// discoverProjects runs server-side vcsRoot+URL filter (one query per unique repo fragment) and a parallel pipelines query for label enrichment, then rejects forks via client-side normalized-URL check.
func discoverProjects(client api.ClientInterface, remoteURLs []string) (*discovery, error) {
	fragments, normRemotes := extractFragments(remoteURLs)
	if len(fragments) == 0 {
		return &discovery{}, nil
	}

	fields := []string{
		"id", "name", "projectId", "projectName", "paused",
		"vcs-root-entries.vcs-root-entry.vcs-root.properties.property.name",
		"vcs-root-entries.vcs-root-entry.vcs-root.properties.property.value",
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		bts       = map[string]*api.BuildType{}
		order     []string
		btErr     error
		pipelines map[string]string
	)

	for frag := range fragments {
		wg.Go(func() {
			resp, err := client.GetBuildTypes(api.BuildTypesOptions{
				VcsRootURL: frag,
				Limit:      500,
				Fields:     fields,
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if btErr == nil {
					btErr = err
				}
				return
			}
			for i := range resp.BuildTypes {
				bt := &resp.BuildTypes[i]
				if !buildTypeMatchesRemotes(bt, normRemotes) {
					continue
				}
				if _, seen := bts[bt.ID]; seen {
					continue
				}
				bts[bt.ID] = bt
				order = append(order, bt.ID)
			}
		})
	}

	if client.SupportsFeature("pipelines") {
		wg.Go(func() {
			resp, err := client.GetPipelines(api.PipelinesOptions{Limit: 10000})
			if err != nil || resp == nil {
				return
			}
			m := make(map[string]string, len(resp.Pipelines))
			for _, p := range resp.Pipelines {
				if p.HeadBuildType != nil && p.HeadBuildType.ID != "" {
					m[p.HeadBuildType.ID] = p.Name
				}
			}
			mu.Lock()
			pipelines = m
			mu.Unlock()
		})
	}

	wg.Wait()
	if btErr != nil {
		return nil, btErr
	}

	byProject := map[string]*projectMatch{}
	var projectOrder []string
	var allJobs []api.BuildType
	for _, id := range order {
		bt := bts[id]
		if bt.Paused {
			continue
		}
		allJobs = append(allJobs, *bt)
		pm, ok := byProject[bt.ProjectID]
		if !ok {
			pm = &projectMatch{ID: bt.ProjectID, Name: bt.ProjectName}
			byProject[bt.ProjectID] = pm
			projectOrder = append(projectOrder, bt.ProjectID)
		}
		pm.Jobs = append(pm.Jobs, *bt)
	}

	projects := make([]projectMatch, 0, len(projectOrder))
	for _, pid := range projectOrder {
		projects = append(projects, *byProject[pid])
	}
	sort.SliceStable(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	sort.SliceStable(allJobs, func(i, j int) bool {
		a, b := allJobs[i], allJobs[j]
		if a.ProjectName != b.ProjectName {
			return a.ProjectName < b.ProjectName
		}
		return a.Name < b.Name
	})

	return &discovery{Projects: projects, AllJobs: allJobs, Pipelines: pipelines}, nil
}

// extractFragments dedup-maps remote fragment→original remote + collects the normalized remote set for post-match filtering.
func extractFragments(remoteURLs []string) (map[string]string, map[string]bool) {
	fragments := map[string]string{}
	norm := map[string]bool{}
	for _, raw := range remoteURLs {
		n := git.CanonicalURL(raw)
		if n == "" {
			continue
		}
		norm[n] = true
		frag := git.RepoPath(raw)
		if frag == "" || frag == n {
			continue
		}
		if _, ok := fragments[frag]; !ok {
			fragments[frag] = raw
		}
	}
	return fragments, norm
}

// buildTypeMatchesRemotes verifies a server-side `matchType:contains` candidate by normalizing its VCS root URL against the user's remote set, rejecting forks.
func buildTypeMatchesRemotes(bt *api.BuildType, norm map[string]bool) bool {
	if bt == nil || bt.VcsRootEntries == nil {
		return false
	}
	for _, e := range bt.VcsRootEntries.VcsRootEntry {
		if e.VcsRoot == nil || e.VcsRoot.Properties == nil {
			continue
		}
		for _, p := range e.VcsRoot.Properties.Property {
			if p.Name == "url" {
				if n := git.CanonicalURL(p.Value); n != "" && norm[n] {
					return true
				}
			}
		}
	}
	return false
}

// pickCwdAffinity returns the index of the project whose ID/name best matches the cwd path hint, else 0; used only to pre-focus the picker.
func pickCwdAffinity(projects []projectMatch, cwdPathHint string) int {
	cwdPathHint = strings.ToLower(strings.Trim(cwdPathHint, "/"))
	if cwdPathHint == "" || len(projects) == 0 {
		return 0
	}
	last := cwdPathHint
	if i := strings.LastIndex(cwdPathHint, "/"); i >= 0 {
		last = cwdPathHint[i+1:]
	}
	for i, p := range projects {
		if strings.Contains(strings.ToLower(p.ID), last) || strings.Contains(strings.ToLower(p.Name), last) {
			return i
		}
	}
	return 0
}
