package onboard

import (
	"fmt"
	"slices"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/git"
)

// vcsRootMatchFields fetches only what URL matching needs; properties aren't in VcsRootFields.Default.
var vcsRootMatchFields = []string{"id", "project.id", "properties.property.name", "properties.property.value"}

// isRepositoryAuthorized reports whether a VCS root usable by the onboarded project points at repo's remote;
// usableFrom is that project and its ancestors, since TeamCity projects can only use roots defined there.
func isRepositoryAuthorized(client api.ClientInterface, repo repository, usableFrom ...string) (bool, error) {
	// Compare in HTTPS form, the one onboarding saves, so Azure DevOps SSH remotes match their roots.
	remote := httpsURL(repo.remoteURL)
	want := git.CanonicalURL(remote)
	if want == "" {
		return false, nil
	}
	// Server-side org/repo substring narrows the list; canonical equality then rejects forks and lookalikes.
	roots, _, err := client.GetVcsRoots(api.VcsRootsOptions{
		URL:    git.RepoPath(remote),
		Fields: vcsRootMatchFields,
	})
	if err != nil {
		return false, fmt.Errorf("list VCS roots: %w", err)
	}
	for _, root := range roots.VcsRoot {
		// A matching root in an unrelated project doesn't give this project repository access.
		if root.Properties == nil || root.Project == nil || !slices.Contains(usableFrom, root.Project.ID) {
			continue
		}
		for _, p := range root.Properties.Property {
			if p.Name == "url" && git.CanonicalURL(p.Value) == want {
				return true, nil
			}
		}
	}
	return false, nil
}
