package job

import (
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

type jobListOptions struct {
	project string
	cmdutil.ListFlags
}

func newJobListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &jobListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List jobs",
		Aliases: []string{"ls"},
		Example: `  teamcity job list
  teamcity job list --project Falcon
  teamcity job list --json
  teamcity job list --json=id,name,webUrl
  teamcity job list --plain
  teamcity job list --plain --no-header`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.BuildTypeFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 30)

	return cmd
}

func (opts *jobListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	jobs, err := client.GetBuildTypes(api.BuildTypesOptions{
		Project: opts.project,
		Limit:   opts.Limit,
		Fields:  fields,
	})
	if err != nil {
		return nil, err
	}

	headers := []string{"ID", "NAME", "PROJECT", "STATUS"}
	var rows [][]string

	for _, j := range jobs.BuildTypes {
		status := output.Green("Active")
		if j.Paused {
			status = output.Faint("Paused")
		}

		rows = append(rows, []string{
			j.ID,
			j.Name,
			j.ProjectName,
			status,
		})
	}

	return &cmdutil.ListResult{
		JSON:     jobs,
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{0, 1, 2}},
		EmptyMsg: "No jobs found",
	}, nil
}

func newJobViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cmdutil.ViewOptions{}
	cmd := &cobra.Command{
		Use:     "view <job-id>",
		Short:   "View job details",
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity job view Falcon_Build
  teamcity job view Falcon_Build --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobView(f, args[0], opts)
		},
	}
	cmdutil.AddViewFlags(cmd, opts)
	return cmd
}

func runJobView(f *cmdutil.Factory, jobID string, opts *cmdutil.ViewOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	buildType, err := client.GetBuildType(jobID)
	if err != nil {
		return err
	}

	if opts.Web {
		return browser.OpenURL(buildType.WebURL)
	}

	if opts.JSON {
		return f.Printer.PrintJSON(buildType)
	}

	f.Printer.PrintViewHeader(buildType.Name, buildType.WebURL, func() {
		f.Printer.PrintField("ID", buildType.ID)
		f.Printer.PrintField("Project", buildType.ProjectName+" ("+buildType.ProjectID+")")

		status := output.Green("Active")
		if buildType.Paused {
			status = output.Faint("Paused")
		}
		f.Printer.PrintField("Status", status)
	})

	return nil
}
