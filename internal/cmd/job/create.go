package job

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/spf13/cobra"
)

type jobCreateOptions struct {
	id       string
	project  string
	template string
	json     bool
	web      bool
}

func newJobCreateCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &jobCreateOptions{}

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a job",
		Long: `Create a new job (build configuration) in a project.

If --id is omitted, TeamCity derives the job ID from the name.
The parent project is taken from --project, the TEAMCITY_PROJECT
environment variable, or the linked project (see 'teamcity link').`,
		Example: `  teamcity job create Build --project MyProject
  teamcity job create Build --project MyProject --id MyProject_Build
  teamcity job create Build --project MyProject --template MyTemplate
  teamcity job create Build --project MyProject --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobCreate(f, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.id, "id", "", "Explicit job ID (default: auto-generated from name)")
	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Parent project ID")
	cmd.Flags().StringVar(&opts.template, "template", "", "Create from an existing template ID")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser after creation")
	cmd.MarkFlagsMutuallyExclusive("json", "web")

	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func runJobCreate(f *cmdutil.Factory, name string, opts *jobCreateOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	projectID := f.ResolveProject(opts.project)
	if projectID == "" {
		return api.Validation(
			"project id is required",
			"Pass --project <id> or run 'teamcity link' to bind this repository to a project",
		)
	}

	req := api.CreateBuildTypeRequest{ID: opts.id, Name: name}
	if opts.template != "" {
		req.Templates = &api.BuildTypeList{BuildTypes: []api.BuildType{{ID: opts.template}}}
	}

	job, err := client.CreateBuildType(projectID, req)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	if opts.web {
		cmdutil.OpenURLOrWarn(f.Printer, job.WebURL)
	}

	if opts.json {
		return f.Printer.PrintJSON(job)
	}

	proj := job.ProjectName
	if proj == "" {
		proj = job.ProjectID
	}
	if proj != "" {
		f.Printer.Success("Created job %q (id: %s) in project %q", job.Name, job.ID, proj)
	} else {
		f.Printer.Success("Created job %q (id: %s)", job.Name, job.ID)
	}
	if job.WebURL != "" {
		_, _ = fmt.Fprintf(f.Printer.Out, "  %s\n", job.WebURL)
	}

	return nil
}
