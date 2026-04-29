package project

import (
	"cmp"
	"fmt"
	"net/url"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

// openBrowser is overridable in tests.
var openBrowser = browser.OpenURL

// authorizeURLSegment maps a providerType to the URL segment of its obtainToken controller.
var authorizeURLSegment = map[string]string{
	"GitHubApp":       "githubapp",
	"GitHub":          "github",
	"GHE":             "github",
	"GitLabCom":       "gitlab",
	"GitLabCEorEE":    "gitlab",
	"BitBucketCloud":  "bitbucket",
	"AzureDevOps":     "azure",
	"JetBrains Space": "space",
}

type connectionAuthorizeOptions struct {
	project string
}

func newConnectionAuthorizeCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &connectionAuthorizeOptions{}

	cmd := &cobra.Command{
		Use:   "authorize <id>",
		Short: "Open a browser to authorize the current TeamCity user against a connection",
		Long: `Run the per-user OAuth flow against an existing OAuth-style connection.

After this completes, the current user has a token stored in TeamCity for the connection.
This is required before the VCS root test-connection endpoint can verify access using
the connection (TeamCity calls upstream as the current user via the App's user OAuth).

Connection types that don't have a per-user OAuth flow (Docker, AWS) error out.`,
		Example: `  teamcity project connection authorize PROJECT_EXT_42 -p Sandbox`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnectionAuthorize(f, opts, args[0])
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")

	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func runConnectionAuthorize(f *cmdutil.Factory, opts *connectionAuthorizeOptions, id string) error {
	client, err := f.Client()
	if err != nil {
		return err
	}
	projectID := cmp.Or(opts.project, "_Root")

	providerType, err := lookupConnectionProviderType(client, projectID, id)
	if err != nil {
		return err
	}
	return openConnectionAuthorize(f, client, projectID, id, providerType)
}

// openConnectionAuthorize opens the per-user OAuth flow for an existing connection in the user's browser.
func openConnectionAuthorize(f *cmdutil.Factory, client api.ClientInterface, projectID, id, providerType string) error {
	seg, ok := authorizeURLSegment[providerType]
	if !ok {
		return api.Validation(
			fmt.Sprintf("connection type %q does not require browser authorization", providerType),
			"Static-credential connections (Docker, AWS) authenticate at use time",
		)
	}

	// repositories.html is the same entry point the "Sign in to GitHub App" UI button uses.
	authorizeURL := fmt.Sprintf("%s/oauth/%s/repositories.html?projectId=%s&connectionId=%s&updateToken=true&showMode=popup",
		client.ServerURL(), seg, url.QueryEscape(projectID), url.QueryEscape(id))

	f.Printer.Info("Opening browser to authorize connection %s as your TeamCity user...", id)
	if err := openBrowser(authorizeURL); err != nil {
		f.Printer.Warn("Could not open browser automatically: %v", err)
		_, _ = fmt.Fprintf(f.Printer.Out, "  Open this URL:\n  %s\n", output.Cyan(authorizeURL))
	}
	f.Printer.Tip("Complete the flow in your browser. The tab closes on success; you can return here then.")
	return nil
}

func lookupConnectionProviderType(client api.ClientInterface, projectID, id string) (string, error) {
	feats, err := client.GetProjectConnections(projectID)
	if err != nil {
		return "", fmt.Errorf("failed to list connections: %w", err)
	}
	for _, feat := range feats.ProjectFeature {
		if feat.ID != id {
			continue
		}
		if feat.Properties == nil {
			return "", fmt.Errorf("connection %s has no properties", id)
		}
		for _, p := range feat.Properties.Property {
			if p.Name == "providerType" {
				return p.Value, nil
			}
		}
		return "", fmt.Errorf("connection %s has no providerType", id)
	}
	return "", fmt.Errorf("connection %s not found in project %s", id, projectID)
}
