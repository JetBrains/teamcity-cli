// Package onboard implements `teamcity onboard`: connect a repository to TeamCity and hand over to an AI agent.
package onboard

import (
	"fmt"

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
		Long: `Exchange a short-lived onboarding token for a TeamCity access token, pick a
repository (current directory, another path, or a demo), check that TeamCity can
access it, and hand over to an AI agent to finish the setup.`,
		Example: `  teamcity onboard https://teamcity.example.com --token <onboarding-token>`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.serverURL = config.NormalizeURL(args[0])
			return runOnboard(f, opts)
		},
	}

	cmd.Flags().StringVar(&opts.token, "token", "", "Short-lived onboarding token")
	_ = cmd.MarkFlagRequired("token")

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

	// TODO: restore once endpoint ships
	// accessToken, err := auth.ExchangeToken(f, opts.serverURL, opts.token)
	// if err != nil {
	// 	return err
	// }
	// Simulates the exchange: --token is accepted but ignored; uses the existing login for serverURL.
	accessToken, _, err := config.GetTokenForServer(opts.serverURL)
	if err != nil {
		return fmt.Errorf("read token for %s: %w", opts.serverURL, err)
	}
	if accessToken == "" {
		return api.Validation(
			fmt.Sprintf("not logged in to %s", opts.serverURL),
			fmt.Sprintf("Run teamcity auth login -s %s first", opts.serverURL),
		)
	}
	client := api.NewClient(opts.serverURL, accessToken, api.WithDebugFunc(f.Printer.Debug), api.WithVersion(version.String())).WithContext(f.Context())

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

	authorized, err := isRepositoryAuthorized(client, repo)
	if err != nil {
		return err
	}
	if !authorized {
		if authorized, err = authorizeRepository(f, client, repo, instanceID, projectID); err != nil {
			return err
		}
	}

	return handOverToAgent(f, client, repo, projectID, authorized)
}
