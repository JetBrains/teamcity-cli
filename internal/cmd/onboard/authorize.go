package onboard

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/git"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

// githubOAuthProvider is the providerType of a GitHub OAuth connection.
const githubOAuthProvider = "GitHub"

// authorizeRepository creates a VCS root for repo in projectID: anonymous for demos, via the instance project's GitHub connection otherwise; false means the user declined.
func authorizeRepository(f *cmdutil.Factory, client api.ClientInterface, repo repository, instanceID, projectID string) (bool, error) {
	if repo.remoteURL == "" {
		return false, nil // see resolveRepository's no-remotes TODO
	}

	if repo.source == sourceDemo {
		f.Printer.Info("Creating a VCS root with anonymous access for %s", output.Cyan(repo.remoteURL))
		return true, createVcsRoot(f, client, projectID, repo.remoteURL, defaultBranch(f.Context(), repo.remoteURL), "", api.Property{Name: "authMethod", Value: "ANONYMOUS"})
	}

	connID, err := findGitHubConnection(client, instanceID)
	if err != nil {
		return false, err
	}
	if connID == "" {
		return false, api.Validation(
			"no GitHub connection found on the server",
			"Add a GitHub OAuth connection in TeamCity, then rerun teamcity onboard",
		)
	}
	conn := &connection{id: connID, projectID: instanceID}

	// TODO: copy pending design review
	proceed := true
	title := fmt.Sprintf("%s isn't connected to TeamCity yet. Create a VCS root using GitHub connection %s?", repo.remoteURL, conn.id)
	if err := cmdutil.Confirm(title, &proceed); err != nil {
		return false, err
	}
	if !proceed {
		return false, nil
	}

	repoURL := httpsURL(repo.remoteURL)
	ok, err := ensureConnectionAccess(f, client, repoURL, conn)
	if err != nil || !ok {
		return false, err
	}
	// Branch lookup uses the original remote so SSH keys work for private repos.
	return true, createVcsRoot(f, client, projectID, repoURL, defaultBranch(f.Context(), repo.remoteURL), conn.id)
}

// httpsURL rewrites SSH remotes to HTTPS, since OAuth connection tokens only authenticate over HTTPS.
func httpsURL(remoteURL string) string {
	if canon := git.CanonicalURL(remoteURL); canon != "" {
		return "https://" + canon + ".git"
	}
	return remoteURL
}

// ensureConnectionAccess tests access through conn, opening the browser authorization until it passes; false means the user gave up.
func ensureConnectionAccess(f *cmdutil.Factory, client api.ClientInterface, repoURL string, conn *connection) (bool, error) {
	if !client.SupportsFeature("vcs_test_connection") {
		return true, nil // older servers: let CreateVcsRoot surface auth problems
	}
	req := api.TestConnectionRequest{URL: repoURL, VcsName: "jetbrains.git", ConnectionID: conn.id}
	for {
		f.Printer.Progress("Checking access to %s... ", output.Cyan(repoURL))
		result, err := client.TestVcsConnection(req, conn.projectID)
		if err != nil {
			f.Printer.Info("%s", output.Red(output.Sym().Cross))
			return false, fmt.Errorf("connection test failed: %w", err)
		}
		if result.Status == "OK" {
			f.Printer.Info("%s", output.Green(output.Sym().Check))
			return true, nil
		}
		f.Printer.Info("%s", output.Red(output.Sym().Cross))

		msg := "connection test failed"
		if len(result.Errors) > 0 {
			msg = result.Errors[0].Message
		}
		// Same signal `project vcs create` uses for a connection the current user hasn't authorized yet.
		if !strings.Contains(msg, "Malformed request") {
			return false, fmt.Errorf("%s", msg)
		}

		cmdutil.OpenURLOrWarn(f.Printer, fmt.Sprintf("%s/oauth/github/repositories.html?projectId=%s&connectionId=%s&updateToken=true&showMode=popup",
			client.ServerURL(), url.QueryEscape(conn.projectID), url.QueryEscape(conn.id)))
		// TODO: copy pending design review
		done := true
		if err := cmdutil.Confirm("Finished authorizing in the browser?", &done); err != nil {
			return false, err
		}
		if !done {
			return false, nil
		}
	}
}

// connection is a GitHub OAuth connection and the instance project that owns it; VCS roots using it must live there or below.
type connection struct {
	id        string
	projectID string
}

// findGitHubConnection returns the ID of a GitHub OAuth connection visible from projectID, or "".
func findGitHubConnection(client api.ClientInterface, projectID string) (string, error) {
	feats, err := client.GetProjectConnections(projectID)
	if err != nil {
		return "", fmt.Errorf("list connections in %s: %w", projectID, err)
	}
	for _, feat := range feats.ProjectFeature {
		if feat.Properties == nil {
			continue
		}
		for _, p := range feat.Properties.Property {
			if p.Name == "providerType" && p.Value == githubOAuthProvider {
				return feat.ID, nil
			}
		}
	}
	return "", nil
}

// createVcsRoot creates a Git VCS root for repoURL in projectID; connID lets the server resolve OAuth credentials.
func createVcsRoot(f *cmdutil.Factory, client api.ClientInterface, projectID, repoURL, branch, connID string, authProps ...api.Property) error {
	props := append([]api.Property{
		{Name: "url", Value: repoURL},
		{Name: "branch", Value: branch},
	}, authProps...)
	created, err := client.CreateVcsRoot(api.VcsRoot{
		Name:         cmp.Or(git.RepoPath(repoURL), repoURL),
		VcsName:      "jetbrains.git",
		Project:      &api.Project{ID: projectID},
		ConnectionID: connID,
		Properties:   &api.PropertyList{Property: props},
	})
	if err != nil {
		return fmt.Errorf("failed to create VCS root: %w", err)
	}
	f.Printer.Success("Created VCS root %q (%s)", created.Name, created.ID)
	return nil
}

// defaultBranch asks the remote for its HEAD ref without a clone; falls back to refs/heads/main.
func defaultBranch(ctx context.Context, remoteURL string) string {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--symref", "--end-of-options", remoteURL, "HEAD")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // fail fast instead of prompting for credentials
	out, err := cmd.Output()
	if err == nil {
		// First line looks like "ref: refs/heads/main\tHEAD".
		for line := range strings.SplitSeq(string(out), "\n") {
			if rest, ok := strings.CutPrefix(line, "ref: "); ok {
				if ref, _, ok := strings.Cut(rest, "\t"); ok {
					return ref
				}
			}
		}
	}
	return "refs/heads/main"
}
