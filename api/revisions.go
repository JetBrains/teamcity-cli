package api

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// RevisionSpec pins one VCS root to a commit or to the fetched head of Branch.
type RevisionSpec struct {
	VcsRootID string `json:"vcs_root"`
	Version   string `json:"version,omitempty"`
	Branch    string `json:"branch,omitempty"`
}

func (c *Client) resolveBuildRevisions(job string, opts RunBuildOptions) ([]Revision, error) {
	if opts.Revision != "" && len(opts.Revisions) > 0 {
		return nil, Validation("cannot mix a bare revision with per-root revisions", "Use either Revision or Revisions")
	}
	entries, err := c.GetVcsRootEntries(job)
	if err != nil {
		return nil, fmt.Errorf("failed to get VCS root entries: %w", err)
	}
	roots := []string{}
	for _, entry := range entries.VcsRootEntry {
		if entry.VcsRoot != nil && entry.VcsRoot.ID != "" {
			roots = append(roots, entry.VcsRoot.ID)
		}
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("build configuration %s has no VCS roots; cannot pin revision", job)
	}
	specs := slices.Clone(opts.Revisions)
	if opts.Revision != "" {
		for _, root := range roots {
			specs = append(specs, RevisionSpec{VcsRootID: root, Version: opts.Revision, Branch: opts.Branch})
		}
	}
	seen := map[string]bool{}
	for _, spec := range specs {
		switch {
		case !slices.Contains(roots, spec.VcsRootID):
			return nil, Validation(fmt.Sprintf("VCS root %q is not attached to %s", spec.VcsRootID, job), "Available VCS roots: "+strings.Join(roots, ", "))
		case seen[spec.VcsRootID]:
			return nil, Validation("duplicate revision for VCS root "+spec.VcsRootID, "Pass each VCS root at most once")
		case spec.Version == "" && spec.Branch == "":
			return nil, Validation("revision or branch required for VCS root "+spec.VcsRootID, "Use ROOT=SHA, ROOT=SHA@BRANCH, or ROOT=@BRANCH")
		}
		seen[spec.VcsRootID] = true
	}
	if slices.ContainsFunc(specs, func(s RevisionSpec) bool { return s.Version == "" }) {
		if err := c.resolveBranchHeads(job, specs); err != nil {
			return nil, err
		}
	}
	revisions := make([]Revision, 0, len(specs))
	for _, spec := range specs {
		revisions = append(revisions, Revision{
			Version: spec.Version, VcsBranchName: revisionBranch(spec.Branch),
			VcsRootInstance: &VcsRootInstanceRef{VcsRootID: spec.VcsRootID},
		})
	}
	return revisions, nil
}

// resolveBranchHeads resolves all branch-only pins in one request without fetching remote Git repositories.
func (c *Client) resolveBranchHeads(job string, specs []RevisionSpec) error {
	type branchHead struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	var state struct {
		Instances []struct {
			Root            string `json:"vcs-root-id"`
			RepositoryState struct {
				Branches []branchHead `json:"branch"`
			} `json:"repositoryState"`
		} `json:"vcs-root-instance"`
	}
	path := "/app/rest/buildTypes/id:" + url.PathEscape(job) + "/vcs-root-instances?fields=vcs-root-instance(vcs-root-id,repositoryState(branch(name,version)))"
	if err := c.get(c.ctx(), path, &state); err != nil {
		return fmt.Errorf("failed to get VCS branch heads: %w", err)
	}
	for i, spec := range specs {
		if spec.Version != "" {
			continue
		}
		for _, inst := range state.Instances {
			if inst.Root != spec.VcsRootID {
				continue
			}
			branches := inst.RepositoryState.Branches
			if j := slices.IndexFunc(branches, func(branch branchHead) bool {
				return branch.Name == revisionBranch(spec.Branch) || branch.Name == spec.Branch
			}); j >= 0 {
				specs[i].Version = branches[j].Version
			}
		}
		if specs[i].Version == "" {
			return Validation(fmt.Sprintf("no fetched revision for branch %q in VCS root %s", spec.Branch, spec.VcsRootID), "Check the root's branch specification, or pass ROOT=SHA@BRANCH")
		}
	}
	return nil
}

// revisionBranch preserves full refs and qualifies short Git branch names.
func revisionBranch(branch string) string {
	if branch == "" || strings.HasPrefix(branch, "refs/") {
		return branch
	}
	return "refs/heads/" + branch
}
