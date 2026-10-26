// Package onboard implements `teamcity onboard`: connect a repository to TeamCity and hand over to an AI agent.
package onboard

import (
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/spf13/cobra"
)

type onboardOptions struct {
	serverURL string
	token     string
}

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &onboardOptions{}
	cmd := &cobra.Command{
		Use:   "onboard <server-url>",
		Short: "Onboard a repository to TeamCity",
		Long: `Exchange a short-lived onboarding token for a TeamCity access token (or use
your existing login when --token is omitted), pick a repository (current directory,
another path, or a sample repository), check that TeamCity can access it, and hand
over to an AI agent to finish the setup.`,
		Example: `  # With an onboarding token generated in TeamCity
  teamcity onboard https://teamcity.example.com --token <onboarding-token>

  # Already logged in with teamcity auth login
  teamcity onboard https://teamcity.example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.serverURL = config.NormalizeURL(args[0])
			return runOnboard(f, opts)
		},
	}

	cmd.Flags().StringVar(&opts.token, "token", "", "Short-lived onboarding token (default: use existing login)")

	cmdutil.MarkExperimental(f, cmd)

	return cmd
}

func runOnboard(f *cmdutil.Factory, opts *onboardOptions) error {
	if !f.IsInteractive() {
		return api.Validation(
			"teamcity onboard requires an interactive terminal",
			"Run it in a terminal without --no-input",
		)
	}

	token, err := accessToken(f, opts.serverURL, opts.token)
	if err != nil {
		return err
	}
	client := api.NewClient(opts.serverURL, token, api.WithDebugFunc(f.Printer.Debug), api.WithVersion(version.String())).WithContext(f.Context())

	repo, err := chooseRepository(f)
	if err != nil {
		return err
	}

	instanceID, err := findInstanceProject(client)
	if err != nil {
		return err
	}
	projectID, err := ensureRepoProject(f, client, instanceID, repo)
	if err != nil {
		return err
	}

	authorized, err := isRepositoryAuthorized(client, repo, projectID, instanceID, rootProjectID)
	if err != nil {
		return err
	}
	if !authorized {
		if authorized, err = authorizeRepository(f, client, repo, instanceID, projectID); err != nil {
			return err
		}
	}

	if repo.source == sourceSample {
		if repo, err = cloneSample(f, repo); err != nil {
			return err
		}
	}

	return handOverToAgent(f, client, repo, projectID, authorized)
}
