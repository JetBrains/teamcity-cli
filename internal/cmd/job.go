package cmd

import (
	"fmt"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/api"
	"github.com/tiulpin/teamcity-cli/internal/output"
)

func newJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Manage jobs (build configurations)",
		Long:  `List and manage TeamCity jobs (build configurations).`,
	}

	cmd.AddCommand(newJobListCmd())
	cmd.AddCommand(newJobViewCmd())
	cmd.AddCommand(newJobPauseCmd())
	cmd.AddCommand(newJobResumeCmd())
	cmd.AddCommand(newParamCmd("job", jobParamAPI))

	return cmd
}

type jobListOptions struct {
	project    string
	limit      int
	jsonFields string
}

func newJobListCmd() *cobra.Command {
	opts := &jobListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List jobs",
		Example: `  tc job list
  tc job list --project Sandbox
  tc job list --json
  tc job list --json=id,name,webUrl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobList(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 30, "Maximum number of jobs")
	AddJSONFieldsFlag(cmd, &opts.jsonFields)

	return cmd
}

func runJobList(cmd *cobra.Command, opts *jobListOptions) error {
	jsonResult, showHelp, err := ParseJSONFields(cmd, opts.jsonFields, &api.BuildTypeFields)
	if err != nil {
		return err
	}
	if showHelp {
		return nil
	}

	client, err := getClient()
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

	widths := output.ColumnWidths(20, 60, 40, 30, 30)

	for _, j := range jobs.BuildTypes {
		status := output.Green("Active")
		if j.Paused {
			status = output.Faint("Paused")
		}

		rows = append(rows, []string{
			output.Truncate(j.ID, widths[0]),
			output.Truncate(j.Name, widths[1]),
			output.Truncate(j.ProjectName, widths[2]),
			status,
		})
	}

	output.PrintTable(headers, rows)
	return nil
}

type jobViewOptions struct {
	json bool
	web  bool
}

func newJobViewCmd() *cobra.Command {
	opts := &jobViewOptions{}

	cmd := &cobra.Command{
		Use:   "view <job-id>",
		Short: "View job details",
		Args:  cobra.ExactArgs(1),
		Example: `  tc job view Sandbox_Demo
  tc job view Sandbox_Demo --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobView(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")

	return cmd
}

func runJobView(jobID string, opts *jobViewOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	buildType, err := client.GetBuildType(jobID)
	if err != nil {
		return err
	}

	if opts.web {
		return browser.OpenURL(buildType.WebURL)
	}

	if opts.json {
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

func newJobPauseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pause <job-id>",
		Short:   "Pause a job",
		Long:    `Pause a job to prevent new runs from being triggered.`,
		Args:    cobra.ExactArgs(1),
		Example: `  tc job pause Sandbox_Demo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobPause(args[0])
		},
	}

	return cmd
}

func runJobPause(jobID string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if err := client.PauseBuildType(jobID); err != nil {
		return fmt.Errorf("failed to pause job: %w", err)
	}

	output.Success("Paused job %s", jobID)
	return nil
}

func newJobResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resume <job-id>",
		Short:   "Resume a paused job (build configuration)",
		Long:    `Resume a paused job (build configuration) to allow new runs (builds).`,
		Args:    cobra.ExactArgs(1),
		Example: `  tc job resume Sandbox_Demo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobResume(args[0])
		},
	}

	return cmd
}

func runJobResume(jobID string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if err := client.ResumeBuildType(jobID); err != nil {
		return fmt.Errorf("failed to resume job: %w", err)
	}

	output.Success("Resumed job %s", jobID)
	return nil
}
