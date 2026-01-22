package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/api"
	"github.com/tiulpin/teamcity-cli/internal/config"
	tcerrors "github.com/tiulpin/teamcity-cli/internal/errors"
	"github.com/tiulpin/teamcity-cli/internal/output"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
		Long:  `List and view TeamCity projects.`,
	}

	cmd.AddCommand(newProjectListCmd())
	cmd.AddCommand(newProjectViewCmd())
	cmd.AddCommand(newProjectTokenCmd())
	cmd.AddCommand(newParamCmd("project", projectParamAPI))

	return cmd
}

type projectListOptions struct {
	parent string
	limit  int
	json   bool
}

func newProjectListCmd() *cobra.Command {
	opts := &projectListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Long:  `List all TeamCity projects.`,
		Example: `  tc project list
  tc project list --parent AiPlatform`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectList(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.parent, "parent", "p", "", "Filter by parent project ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 100, "Maximum number of projects")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runProjectList(opts *projectListOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	projects, err := client.GetProjects(api.ProjectsOptions{
		Parent: opts.parent,
		Limit:  opts.limit,
	})
	if err != nil {
		return err
	}

	if opts.json {
		return output.PrintJSON(projects)
	}

	if projects.Count == 0 {
		fmt.Println("No projects found")
		return nil
	}

	headers := []string{"ID", "NAME", "PARENT"}
	var rows [][]string

	widths := output.ColumnWidths(20, 40, 33, 34, 33)

	for _, p := range projects.Projects {
		parent := "-"
		if p.ParentProjectID != "" {
			parent = p.ParentProjectID
		}

		rows = append(rows, []string{
			output.Truncate(p.ID, widths[0]),
			output.Truncate(p.Name, widths[1]),
			output.Truncate(parent, widths[2]),
		})
	}

	output.PrintTable(headers, rows)
	return nil
}

type projectViewOptions struct {
	json bool
	web  bool
}

func newProjectViewCmd() *cobra.Command {
	opts := &projectViewOptions{}

	cmd := &cobra.Command{
		Use:   "view <project-id>",
		Short: "View project details",
		Long:  `View details of a TeamCity project.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc project view Sandbox
  tc project view Sandbox --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectView(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")

	return cmd
}

func runProjectView(projectID string, opts *projectViewOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	project, err := client.GetProject(projectID)
	if err != nil {
		return err
	}

	if opts.web {
		return browser.OpenURL(project.WebURL)
	}

	if opts.json {
		return output.PrintJSON(project)
	}

	fmt.Printf("%s\n", output.Cyan(project.Name))
	fmt.Printf("ID: %s\n", project.ID)

	if project.ParentProjectID != "" {
		fmt.Printf("Parent: %s\n", project.ParentProjectID)
	}

	if project.Description != "" {
		fmt.Printf("Description: %s\n", project.Description)
	}

	fmt.Printf("\n%s %s\n", output.Faint("View in browser:"), output.Green(project.WebURL))

	return nil
}

// Token command - manage secure tokens for versioned settings
func newProjectTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage secure tokens",
		Long: `Manage secure tokens for versioned settings.

Secure tokens allow you to store sensitive values (passwords, API keys, etc.)
in TeamCity's credentials storage. The scrambled token can be safely committed
to version control and used in configuration files as credentialsJSON:<token>.

See: https://www.jetbrains.com/help/teamcity/storing-project-settings-in-version-control.html#Managing+Tokens`,
	}

	cmd.AddCommand(newProjectTokenPutCmd())
	cmd.AddCommand(newProjectTokenGetCmd())

	return cmd
}

type projectTokenPutOptions struct {
	stdin bool
}

func newProjectTokenPutCmd() *cobra.Command {
	opts := &projectTokenPutOptions{}

	cmd := &cobra.Command{
		Use:   "put <project-id> [value]",
		Short: "Store a secret and get a secure token",
		Long: `Store a sensitive value and get a secure token reference.

The returned token can be used in versioned settings configuration files
as credentialsJSON:<token>. The actual value is stored securely in TeamCity
and is not committed to version control.

Requires EDIT_PROJECT permission (Project Administrator role).`,
		Example: `  # Store a secret interactively (prompts for value)
  tc project token put Sandbox

  # Store a secret from a value
  tc project token put Sandbox "my-secret-password"

  # Store a secret from stdin (useful for piping)
  echo -n "my-secret" | tc project token put Sandbox --stdin

  # Use the token in versioned settings
  # password: credentialsJSON:<returned-token>`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var value string
			if len(args) > 1 {
				value = args[1]
			}
			return runProjectTokenPut(args[0], value, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.stdin, "stdin", false, "Read value from stdin")

	return cmd
}

func runProjectTokenPut(projectID, value string, opts *projectTokenPutOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if opts.stdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		value = strings.TrimSuffix(string(data), "\n")
	}

	if value == "" {
		prompt := &survey.Password{
			Message: "Enter secure value to scramble:",
		}
		if err := survey.AskOne(prompt, &value); err != nil {
			return fmt.Errorf("failed to read value: %w", err)
		}
	}

	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}

	token, err := client.CreateSecureToken(projectID, value)
	if err != nil {
		return fmt.Errorf("failed to create secure token: %w", err)
	}

	fmt.Println(token)

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, output.Faint("Use in versioned settings as: credentialsJSON:"+token))

	return nil
}

func newProjectTokenGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <project-id> <token>",
		Short: "Get the value of a secure token",
		Long: `Retrieve the original value for a secure token.

This operation requires CHANGE_SERVER_SETTINGS permission,
which is only available to System Administrators.`,
		Example: `  tc project token get Sandbox "credentialsJSON:abc123..."
  tc project token get Sandbox "abc123..."`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectTokenGet(args[0], args[1])
		},
	}

	return cmd
}

func runProjectTokenGet(projectID, token string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	// Strip credentialsJSON: prefix if present
	token = strings.TrimPrefix(token, "credentialsJSON:")

	value, err := client.GetSecureValue(projectID, token)
	if err != nil {
		return fmt.Errorf("failed to get secure value: %w", err)
	}

	fmt.Println(value)
	return nil
}

// getClient is a shared helper for all commands
func getClient() (*api.Client, error) {
	serverURL := config.GetServerURL()
	token := config.GetToken()

	if serverURL == "" || token == "" {
		return nil, tcerrors.NotAuthenticated()
	}

	return api.NewClient(serverURL, token), nil
}
