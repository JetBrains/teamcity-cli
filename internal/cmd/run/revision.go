package run

import (
	"fmt"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
)

// parseRevisionFlags turns --revision values into revision specs: a bare "SHA" (or @head) applies to every VCS root, "ROOT=SHA[@BRANCH]" or "ROOT=@BRANCH" pins one root.
func parseRevisionFlags(values []string) ([]api.RevisionSpec, error) {
	specs := make([]api.RevisionSpec, 0, len(values))
	seen := make(map[string]bool)
	var bare, keyed bool
	for _, raw := range values {
		spec, err := parseRevisionFlag(raw)
		if err != nil {
			return nil, err
		}
		if spec.VcsRootID == "" {
			if bare {
				return nil, api.Validation(
					"only one bare --revision value is allowed",
					"Use ROOT=SHA[@BRANCH] to pin individual VCS roots",
				)
			}
			bare = true
		} else {
			if seen[spec.VcsRootID] {
				return nil, api.Validation(
					fmt.Sprintf("duplicate --revision for VCS root %q", spec.VcsRootID),
					"Pass each VCS root at most once",
				)
			}
			seen[spec.VcsRootID] = true
			keyed = true
		}
		specs = append(specs, spec)
	}
	if bare && keyed {
		return nil, api.Validation(
			"cannot mix a bare --revision with ROOT= forms",
			"Pin roots individually with ROOT=SHA[@BRANCH], or use a single bare SHA for all roots",
		)
	}
	return specs, nil
}

// parseRevisionFlag parses one --revision value; bare values resolve @head and short SHAs via the local git repository, keyed values pass verbatim.
func parseRevisionFlag(raw string) (api.RevisionSpec, error) {
	root, value, found := strings.Cut(raw, "=")
	if !found {
		version, err := resolveRevisionFlag(raw)
		if err != nil {
			return api.RevisionSpec{}, err
		}
		return api.RevisionSpec{Version: version}, nil
	}
	version, branch, hasBranch := strings.Cut(value, "@")
	if root == "" || (version == "" && branch == "") || (hasBranch && branch == "") {
		return api.RevisionSpec{}, api.Validation(
			fmt.Sprintf("invalid --revision value %q", raw),
			"Use ROOT=SHA, ROOT=SHA@BRANCH, or ROOT=@BRANCH",
		)
	}
	return api.RevisionSpec{VcsRootID: root, Version: version, Branch: branch}, nil
}

// formatRevisionSpec renders a keyed spec back in ROOT=SHA[@BRANCH] flag form.
func formatRevisionSpec(s api.RevisionSpec) string {
	v := s.Version
	if s.Branch != "" {
		v += "@" + s.Branch
	}
	return s.VcsRootID + "=" + v
}
