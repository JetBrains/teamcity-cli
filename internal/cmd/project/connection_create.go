package project

import (
	"cmp"
	"fmt"
	"os"
	"strconv"
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
	appName        string
	owner          string
	appID          string
	clientID       string
	clientSecret   string
	stdin          bool
	privateKeyFile string
	noManifest     bool
	noAuthorize    bool
}

func newConnectionCreateGitHubAppCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &githubAppOptions{}

	cmd := &cobra.Command{
		Use:   "github-app",
		Short: "Create a GitHub App connection",
		Long: `Register a GitHub App's credentials in a TeamCity project.

A GitHub App authenticates as itself (no per-user token), making it the right
choice for CI workflows that don't need user-context OAuth.

In interactive mode, the CLI registers a new App for you via GitHub's manifest
flow: it opens a browser, you click "Create", and the credentials are captured
automatically. Use --no-manifest to enter existing credentials manually.`,
		Example: `  # Interactive — registers a new App via the manifest flow
  teamcity project connection create github-app -p Backend
  teamcity project connection create github-app -p Backend --owner my-org

  # Manual mode (skip manifest, supply existing credentials)
  teamcity project connection create github-app -p Backend --no-manifest \
      --name "Backend" --app-id 1234567 --client-id Iv1.abc \
      --private-key-file ./key.pem --stdin <<<"$SECRET"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnectionCreateGitHubApp(f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmd.Flags().StringVar(&opts.name, "name", "", "Connection display name in TeamCity (default: \"GitHub App\")")
	cmd.Flags().StringVar(&opts.appName, "app-name", "", "GitHub App name; must be globally unique on github.com (default: TC <project>@<host>)")
	cmd.Flags().StringVar(&opts.owner, "owner", "", "GitHub organization (interactive: posts the manifest under this org)")
	cmd.Flags().StringVar(&opts.appID, "app-id", "", "GitHub App ID (manual mode)")
	cmd.Flags().StringVar(&opts.clientID, "client-id", "", "GitHub App Client ID (manual mode)")
	cmd.Flags().StringVar(&opts.clientSecret, "client-secret", "", "GitHub App Client Secret (manual mode; prefer --stdin)")
	cmd.Flags().BoolVar(&opts.stdin, "stdin", false, "Read client secret from stdin (manual mode)")
	cmd.Flags().StringVar(&opts.privateKeyFile, "private-key-file", "", "Path to GitHub App private key (manual mode)")
	cmd.Flags().BoolVar(&opts.noManifest, "no-manifest", false, "Skip the manifest flow; collect credentials manually")
	cmd.Flags().BoolVar(&opts.noAuthorize, "no-authorize", false, "Skip the post-create authorize prompt")

	return cmd
}

func runConnectionCreateGitHubApp(f *cmdutil.Factory, opts *githubAppOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	projectID := cmp.Or(opts.project, "_Root")
	interactive := f.IsInteractive()
	useManifest := interactive && !opts.noManifest && opts.appID == ""

	name, err := resolveGitHubAppName(f, opts.name, interactive)
	if err != nil {
		return err
	}

	var (
		appID, clientID, clientSecret, privateKey string
		ownerURL, webhookSecret, installSlug      string
	)

	if useManifest {
		owner := opts.owner
		if owner == "" {
			if err := cmdutil.PromptOptionalString(f.Printer, "GitHub organization (leave empty for personal account)", "", &owner); err != nil {
				return err
			}
		}
		appName := githubAppName(cmp.Or(opts.appName, defaultGitHubAppName(projectID, client.ServerURL())))
		creds, err := runGitHubAppManifestFlow(f.Context(), f.Printer, client.ServerURL(), appName, projectID, owner)
		if err != nil {
			return err
		}
		appID = strconv.FormatInt(creds.AppID, 10)
		clientID = creds.ClientID
		clientSecret = creds.ClientSecret
		privateKey = creds.PEM
		ownerURL = creds.Owner.HTMLURL
		webhookSecret = creds.WebhookSecret
		installSlug = creds.Slug
	} else {
		if interactive {
			f.Printer.Tip("%s", output.TipRegisterGitHubApp(opts.owner))
			_, _ = fmt.Fprintln(f.Printer.Out)
		}
		appID, clientID, clientSecret, privateKey, err = collectGitHubAppCredsManually(f, opts, interactive)
		if err != nil {
			return err
		}
		switch {
		case opts.owner != "":
			ownerURL = "https://github.com/" + opts.owner
		case interactive:
			var ownerLogin string
			if err := cmdutil.PromptOptionalString(f.Printer, "GitHub user/org login (optional)", "The account that owns the App, e.g. my-org", &ownerLogin); err != nil {
				return err
			}
			if ownerLogin != "" {
				ownerURL = "https://github.com/" + ownerLogin
			}
		}
	}

	// gitHubApp.* prefix + connectionSubtype are required by the All-in-one Edit form.
	// useUniqueRedirect=false: manifest flow can only register one callback URL up front.
	props := []api.Property{
		{Name: "providerType", Value: "GitHubApp"},
		{Name: "connectionSubtype", Value: "gitHubApp"},
		{Name: "displayName", Value: name},
		{Name: "gitHubApp.appId", Value: appID},
		{Name: "gitHubApp.clientId", Value: clientID},
		{Name: "secure:gitHubApp.clientSecret", Value: clientSecret},
		{Name: "secure:gitHubApp.privateKey", Value: privateKey},
		{Name: "useUniqueRedirect", Value: "false"},
	}
	if ownerURL != "" {
		props = append(props, api.Property{Name: "gitHubApp.ownerUrl", Value: ownerURL})
	}
	if webhookSecret != "" {
		props = append(props, api.Property{Name: "secure:gitHubApp.webhookSecret", Value: webhookSecret})
	}
	feat := api.ProjectFeature{
		Type:       "OAuthProvider",
		Properties: &api.PropertyList{Property: props},
	}

	created, err := client.CreateProjectFeature(projectID, feat)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	f.Printer.Success("Created connection %q (%s) in project %s", name, created.ID, projectID)

	authorizeDone := false
	if interactive && !opts.noAuthorize {
		ask := true
		if err := cmdutil.Confirm("Authorize as your TeamCity user now?", &ask); err != nil {
			return err
		}
		if ask {
			if err := openConnectionAuthorize(f, client, projectID, created.ID, "GitHubApp"); err != nil {
				f.Printer.Warn("Could not start authorize flow: %v", err)
			} else {
				authorizeDone = true
			}
		}
	}

	printGitHubAppNextSteps(f, projectID, created.ID, installSlug, authorizeDone)
	return nil
}

func printGitHubAppNextSteps(f *cmdutil.Factory, projectID, connectionID, installSlug string, authorizeDone bool) {
	type step struct{ label, value string }
	var steps []step
	if !authorizeDone {
		steps = append(steps, step{"Authorize as user:", fmt.Sprintf("teamcity project connection authorize %s -p %s", connectionID, projectID)})
	}
	if installSlug != "" {
		steps = append(steps, step{"Install on a repo:", "https://github.com/apps/" + installSlug + "/installations/new"})
	}
	steps = append(steps, step{"Create a VCS root:", fmt.Sprintf("teamcity project vcs create -p %s --auth token --connection-id %s --url ...", projectID, connectionID)})

	_, _ = fmt.Fprintln(f.Printer.Out)
	_, _ = fmt.Fprintln(f.Printer.Out, "Next steps:")
	for i, s := range steps {
		_, _ = fmt.Fprintf(f.Printer.Out, "  %d. %s %s\n", i+1, s.label, output.Cyan(s.value))
	}
}

func resolveGitHubAppName(f *cmdutil.Factory, name string, interactive bool) (string, error) {
	if name != "" {
		return name, nil
	}
	name = "GitHub App"
	if !interactive {
		return name, nil
	}
	if err := cmdutil.PromptString(f.Printer, "Connection name", "", &name); err != nil {
		return "", err
	}
	return name, nil
}

func collectGitHubAppCredsManually(f *cmdutil.Factory, opts *githubAppOptions, interactive bool) (appID, clientID, clientSecret, privateKey string, err error) {
	appID = opts.appID
	if appID == "" {
		if !interactive {
			return "", "", "", "", api.RequiredFlag("app-id")
		}
		if err := cmdutil.PromptString(f.Printer, "GitHub App ID", "Numeric ID from the App settings page", &appID); err != nil {
			return "", "", "", "", err
		}
	}
	clientID = opts.clientID
	if clientID == "" {
		if !interactive {
			return "", "", "", "", api.RequiredFlag("client-id")
		}
		if err := cmdutil.PromptString(f.Printer, "Client ID", "Starts with Iv1.", &clientID); err != nil {
			return "", "", "", "", err
		}
	}
	clientSecret, err = resolveSecret(f, opts.clientSecret, opts.stdin, interactive, "Client secret", "client-secret")
	if err != nil {
		return "", "", "", "", err
	}
	privateKey, err = resolvePrivateKey(f, opts.privateKeyFile, interactive)
	if err != nil {
		return "", "", "", "", err
	}
	return appID, clientID, clientSecret, privateKey, nil
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
		secret, err := readStdin(f)
		if err != nil {
			return "", err
		}
		if secret == "" {
			return "", api.Validation(
				label+" is required",
				"Pipe the secret on stdin (got empty input), or use --"+flagName,
			)
		}
		return secret, nil
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
