package onboard

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/git"
)

// rootProjectID is the server root; the instance project is its direct child.
const rootProjectID = "_Root"

// findInstanceProject returns the TeamCity Cloud instance project, the direct child of the server root.
func findInstanceProject(client api.ClientInterface) (string, error) {
	children, _, err := client.GetProjects(api.ProjectsOptions{Parent: rootProjectID, ExcludeArchived: true, Fields: []string{"id"}})
	if err != nil {
		return "", fmt.Errorf("list projects in %s: %w", rootProjectID, err)
	}
	if len(children.Projects) == 0 {
		return "", api.Validation("no instance project found under the server root", "teamcity onboard supports TeamCity Cloud instances")
	}
	// TODO: handle multiple root children
	return children.Projects[0].ID, nil
}

// ensureRepoProject returns the ID of the project named after repo under instanceID, creating it when missing.
func ensureRepoProject(f *cmdutil.Factory, client api.ClientInterface, instanceID string, repo repository) (string, error) {
	name := repoName(repo)
	children, _, err := client.GetProjects(api.ProjectsOptions{Parent: instanceID, Fields: []string{"id", "name"}})
	if err != nil {
		return "", fmt.Errorf("list projects in %s: %w", instanceID, err)
	}
	for _, p := range children.Projects {
		if p.Name == name {
			return p.ID, nil
		}
	}
	created, err := client.CreateProject(api.CreateProjectRequest{Name: name, ParentProject: &api.ProjectRef{ID: instanceID}})
	if err != nil {
		return "", fmt.Errorf("failed to create project %q: %w", name, err)
	}
	f.Printer.Success("Created project %q (%s)", created.Name, created.ID)
	return created.ID, nil
}

// repoName is the last path segment of the remote (acme/app → app), or the local directory name without a remote.
func repoName(repo repository) string {
	if p := git.RepoPath(repo.remoteURL); p != "" {
		return path.Base(p)
	}
	if repo.remoteURL != "" {
		return path.Base(strings.TrimSuffix(repo.remoteURL, ".git"))
	}
	return filepath.Base(repo.dir)
}
