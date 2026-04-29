package project

import (
	"cmp"
	"fmt"
	"os"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func newConnectionCreateCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project connection",
		Long: `Create a connection to an external service (GitHub App, Docker registry, ...).

Each provider has its own subcommand because property names differ.

See: https://www.jetbrains.com/help/teamcity/configuring-connections.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newConnectionCreateGitHubAppCmd(f))
	cmd.AddCommand(newConnectionCreateDockerCmd(f))

	return cmd
}

type githubAppOptions struct {
	project        string
	name           string
	owner          string
	appID          string
	clientID       string
	clientSecret   string
	stdin          bool
	privateKeyFile string
}

func newConnectionCreateGitHubAppCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &githubAppOptions{}

	cmd := &cobra.Command{
		Use:   "github-app",
		Short: "Create a GitHub App connection",
		Long: `Register a GitHub App's credentials in a TeamCity project.

A GitHub App authenticates as itself (no per-user token), making it the right
choice for CI workflows that don't need user-context OAuth.`,
		Example: `  # Interactive
  teamcity project connection create github-app -p Backend

  # Non-interactive (read client secret from stdin)
  teamcity project connection create github-app -p Backend \
      --name "Backend" --app-id 1234567 --client-id Iv1.abc \
      --private-key-file ./key.pem --stdin <<<"$SECRET"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnectionCreateGitHubApp(f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmd.Flags().StringVar(&opts.name, "name", "", "Display name")
	cmd.Flags().StringVar(&opts.owner, "owner", "", "GitHub user/org (used in the registration tip only)")
	cmd.Flags().StringVar(&opts.appID, "app-id", "", "GitHub App ID")
	cmd.Flags().StringVar(&opts.clientID, "client-id", "", "GitHub App Client ID (Iv1...)")
	cmd.Flags().StringVar(&opts.clientSecret, "client-secret", "", "GitHub App Client Secret (prefer --stdin)")
	cmd.Flags().BoolVar(&opts.stdin, "stdin", false, "Read client secret from stdin")
	cmd.Flags().StringVar(&opts.privateKeyFile, "private-key-file", "", "Path to GitHub App private key (.pem)")

	return cmd
}

func runConnectionCreateGitHubApp(f *cmdutil.Factory, opts *githubAppOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	projectID := cmp.Or(opts.project, "_Root")
	interactive := f.IsInteractive()

	if interactive && opts.appID == "" {
		f.Printer.Tip("%s", output.TipRegisterGitHubApp(opts.owner))
		_, _ = fmt.Fprintln(f.Printer.Out)
	}

	name := opts.name
	if name == "" {
		if !interactive {
			return api.RequiredFlag("name")
		}
		if err := cmdutil.PromptString(f.Printer, "Connection name", "", &name); err != nil {
			return err
		}
	}

	appID := opts.appID
	if appID == "" {
		if !interactive {
			return api.RequiredFlag("app-id")
		}
		if err := cmdutil.PromptString(f.Printer, "GitHub App ID", "Numeric ID from the App settings page", &appID); err != nil {
			return err
		}
	}

	clientID := opts.clientID
	if clientID == "" {
		if !interactive {
			return api.RequiredFlag("client-id")
		}
		if err := cmdutil.PromptString(f.Printer, "Client ID", "Starts with Iv1.", &clientID); err != nil {
			return err
		}
	}

	clientSecret, err := resolveSecret(f, opts.clientSecret, opts.stdin, interactive, "Client secret", "client-secret")
	if err != nil {
		return err
	}

	privateKey, err := resolvePrivateKey(f, opts.privateKeyFile, interactive)
	if err != nil {
		return err
	}

	feat := api.ProjectFeature{
		Type: "OAuthProvider",
		Properties: &api.PropertyList{
			Property: []api.Property{
				{Name: "providerType", Value: "GitHubApp"},
				{Name: "displayName", Value: name},
				{Name: "appId", Value: appID},
				{Name: "clientId", Value: clientID},
				{Name: "secure:clientSecret", Value: clientSecret},
				{Name: "secure:privateKey", Value: privateKey},
			},
		},
	}

	created, err := client.CreateProjectFeature(projectID, feat)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	f.Printer.Success("Created connection %q (%s) in project %s", name, created.ID, projectID)
	return nil
}

type dockerOptions struct {
	project  string
	name     string
	url      string
	username string
	password string
	stdin    bool
}

func newConnectionCreateDockerCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &dockerOptions{}

	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Create a Docker registry connection",
		Long: `Register Docker registry credentials in a TeamCity project.

Stores a long-lived password — prefer a service account / robot user
over a personal account.`,
		Example: `  # Interactive
  teamcity project connection create docker -p Backend

  # Non-interactive (read password from stdin)
  echo "$DOCKER_TOKEN" | teamcity project connection create docker -p Backend \
      --name GHCR --url https://ghcr.io --username my-org --stdin`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnectionCreateDocker(f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmd.Flags().StringVar(&opts.name, "name", "", "Display name")
	cmd.Flags().StringVar(&opts.url, "url", "", "Registry URL (e.g. https://ghcr.io)")
	cmd.Flags().StringVar(&opts.username, "username", "", "Registry username")
	cmd.Flags().StringVar(&opts.password, "password", "", "Registry password (prefer --stdin)")
	cmd.Flags().BoolVar(&opts.stdin, "stdin", false, "Read password from stdin")

	return cmd
}

func runConnectionCreateDocker(f *cmdutil.Factory, opts *dockerOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	projectID := cmp.Or(opts.project, "_Root")
	interactive := f.IsInteractive()

	if err := promptIfEmpty(f, &opts.name, "Connection name", "", interactive, "name"); err != nil {
		return err
	}
	if err := promptIfEmpty(f, &opts.url, "Registry URL", "e.g. https://ghcr.io", interactive, "url"); err != nil {
		return err
	}
	if err := promptIfEmpty(f, &opts.username, "Username", "", interactive, "username"); err != nil {
		return err
	}

	password, err := resolveSecret(f, opts.password, opts.stdin, interactive, "Password", "password")
	if err != nil {
		return err
	}

	feat := api.ProjectFeature{
		Type: "OAuthProvider",
		Properties: &api.PropertyList{
			Property: []api.Property{
				{Name: "providerType", Value: "Docker"},
				{Name: "displayName", Value: opts.name},
				{Name: "repositoryUrl", Value: opts.url},
				{Name: "userName", Value: opts.username},
				{Name: "secure:userPass", Value: password},
			},
		},
	}

	created, err := client.CreateProjectFeature(projectID, feat)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	f.Printer.Success("Created connection %q (%s) in project %s", opts.name, created.ID, projectID)
	f.Printer.Tip("%s", output.TipDockerServiceAccount)
	return nil
}

func promptIfEmpty(f *cmdutil.Factory, value *string, title, description string, interactive bool, flagName string) error {
	if *value != "" {
		return nil
	}
	if !interactive {
		return api.RequiredFlag(flagName)
	}
	return cmdutil.PromptString(f.Printer, title, description, value)
}

func resolveSecret(f *cmdutil.Factory, value string, stdin, interactive bool, label, flagName string) (string, error) {
	if value != "" {
		return value, nil
	}
	if stdin {
		return readStdin(f)
	}
	if !interactive {
		return "", api.Validation(
			label+" is required",
			"Provide --"+flagName+", --stdin, or run interactively",
		)
	}
	if err := cmdutil.PromptSecret(label, &value); err != nil {
		return "", err
	}
	return value, nil
}

func resolvePrivateKey(f *cmdutil.Factory, path string, interactive bool) (string, error) {
	if path == "" {
		if !interactive {
			return "", api.RequiredFlag("private-key-file")
		}
		if err := cmdutil.PromptString(f.Printer, "Private key path (.pem)", "", &path); err != nil {
			return "", err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read private key: %w", err)
	}
	pem := strings.TrimSpace(string(data))
	if pem == "" {
		return "", api.Validation(
			"private key file is empty",
			fmt.Sprintf("Verify that %s contains a PEM-encoded private key", path),
		)
	}
	return pem, nil
}
