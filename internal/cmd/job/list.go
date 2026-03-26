package job

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

type jobListOptions struct {
	project    string
	limit      int
	jsonFields string
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
  teamcity job list --json=id,name,webUrl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobList(f, cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 30, "Maximum number of jobs")
	cmdutil.AddJSONFieldsFlag(cmd, &opts.jsonFields)

	return cmd
}

func runJobList(f *cmdutil.Factory, cmd *cobra.Command, opts *jobListOptions) error {
	if err := cmdutil.ValidateLimit(opts.limit); err != nil {
		return err
	}
	jsonResult, showHelp, err := cmdutil.ParseJSONFields(cmd, opts.jsonFields, &api.BuildTypeFields)
	if err != nil {
		return err
	}
	if showHelp {
		return nil
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	jobs, err := client.GetBuildTypes(api.BuildTypesOptions{
		Project: opts.project,
		Limit:   opts.limit,
		Fields:  jsonResult.Fields,
	})
	if err != nil {
		return err
	}

	if jsonResult.Enabled {
		return output.PrintJSON(jobs)
	}

	if jobs.Count == 0 {
		fmt.Println("No jobs found")
		return nil
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

	output.AutoSizeColumns(headers, rows, 2, 0, 1, 2)
	output.PrintTable(headers, rows)
	return nil
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
		return output.PrintJSON(buildType)
	}

	fmt.Printf("%s\n", output.Cyan(buildType.Name))
	fmt.Printf("ID: %s\n", buildType.ID)
	fmt.Printf("Project: %s (%s)\n", buildType.ProjectName, buildType.ProjectID)

	status := output.Green("Active")
	if buildType.Paused {
		status = output.Faint("Paused")
	}
	fmt.Printf("Status: %s\n", status)

	fmt.Printf("\n%s %s\n", output.Faint("View in browser:"), output.Green(buildType.WebURL))

	return nil
}
