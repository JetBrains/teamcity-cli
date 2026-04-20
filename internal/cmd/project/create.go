package project

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

type projectCreateOptions struct {
	id     string
	parent string
	json   bool
	web    bool
}

func newProjectCreateCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &projectCreateOptions{}

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a project",
		Long: `Create a new TeamCity project.

If --id is omitted, TeamCity derives the project ID from the name.
If --parent is omitted, the project is created under the Root project.`,
		Example: `  teamcity project create MyProject
  teamcity project create MyProject --id MyProject
  teamcity project create MyProject --parent ParentProject
  teamcity project create MyProject --id MyProject --parent ParentProject
  teamcity project create MyProject --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f.JSONOutput = opts.json
			return runProjectCreate(f, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.id, "id", "", "Explicit project ID (default: auto-generated from name)")
	cmd.Flags().StringVarP(&opts.parent, "parent", "p", "", "Parent project ID (default: _Root)")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser after creation")

	return cmd
}

func runProjectCreate(f *cmdutil.Factory, name string, opts *projectCreateOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	req := api.CreateProjectRequest{
		ID:              opts.id,
		Name:            name,
		ParentProjectID: opts.parent,
	}

	project, err := client.CreateProject(req)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	if opts.web && project.WebURL != "" {
		if err := browser.OpenURL(project.WebURL); err != nil {
			return err
		}
	}

	if opts.json {
		return f.Printer.PrintJSON(project)
	}

	f.Printer.Success("Created project %q (id: %s)", project.Name, project.ID)
	if project.WebURL != "" {
		_, _ = fmt.Fprintf(f.Printer.Out, "  %s\n", project.WebURL)
	}

	return nil
}
