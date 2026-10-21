package onboard

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/git"
)

// vcsRootMatchFields fetches only what URL matching needs; properties aren't in VcsRootFields.Default.
var vcsRootMatchFields = []string{"id", "project.id", "properties.property.name", "properties.property.value"}

// isRepositoryAuthorized reports whether a VCS root visible to the user points at repo's remote.
func isRepositoryAuthorized(client api.ClientInterface, repo repository) (bool, error) {
	want := git.CanonicalURL(repo.remoteURL)
	if want == "" {
		return false, nil
	}
	// Server-side org/repo substring narrows the list; canonical equality then rejects forks and lookalikes.
	roots, _, err := client.GetVcsRoots(api.VcsRootsOptions{
		URL:    git.RepoPath(repo.remoteURL),
		Fields: vcsRootMatchFields,
	})
	if err != nil {
		return false, fmt.Errorf("list VCS roots: %w", err)
	}
	for _, root := range roots.VcsRoot {
		if root.Properties == nil {
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
