package run

import (
	"fmt"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
)

// parseRevisionFlags keeps bare revisions local and per-root revisions independent of the local checkout.
func parseRevisionFlags(values []string) (string, []api.RevisionSpec, error) {
	specs := []api.RevisionSpec{}
	seen := map[string]bool{}
	for _, raw := range values {
		root, value, keyed := strings.Cut(raw, "=")
		if !keyed {
			if len(values) != 1 {
				return "", nil, api.Validation("cannot mix a bare --revision with other revisions", "Use a single SHA or repeat ROOT=SHA[@BRANCH]")
			}
			revision, err := resolveRevisionFlag(raw)
			return revision, specs, err
		}
		version, branch, hasBranch := strings.Cut(value, "@")
		if root == "" || value == "" || (hasBranch && branch == "") {
			return "", nil, api.Validation(fmt.Sprintf("invalid --revision value %q", raw), "Use ROOT=SHA, ROOT=SHA@BRANCH, or ROOT=@BRANCH")
		}
		if seen[root] {
			return "", nil, api.Validation("duplicate --revision for VCS root "+root, "Pass each VCS root at most once")
		}
		seen[root] = true
		specs = append(specs, api.RevisionSpec{VcsRootID: root, Version: version, Branch: branch})
	}
	return "", specs, nil
}
