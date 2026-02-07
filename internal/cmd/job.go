package cmd

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func newJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Manage jobs (build configurations)",
		Long:  `List and manage TeamCity jobs (build configurations).`,
		Args:  cobra.NoArgs,
		RunE:  subcommandRequired,
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
  tc job list --project Falcon
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
	if err := validateLimit(opts.limit); err != nil {
		return err
	}
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

func newJobViewCmd() *cobra.Command {
	opts := &viewOptions{}
	cmd := &cobra.Command{
		Use:   "view <job-id>",
		Short: "View job details",
		Args:  cobra.ExactArgs(1),
		Example: `  tc job view Falcon_Build
  tc job view Falcon_Build --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobView(args[0], opts)
		},
	}
	addViewFlags(cmd, opts)
	return cmd
}

func runJobView(jobID string, opts *viewOptions) error {
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

type jobStateAction struct {
	use    string
	short  string
	long   string
	verb   string
	paused bool
}

var jobStateActions = map[string]jobStateAction{
	"pause":  {"pause", "Pause a job", "Pause a job to prevent new runs from being triggered.", "Paused", true},
	"resume": {"resume", "Resume a paused job", "Resume a paused job to allow new runs.", "Resumed", false},
}

func newJobStateCmd(a jobStateAction) *cobra.Command {
	return &cobra.Command{
		Use:     fmt.Sprintf("%s <job-id>", a.use),
		Short:   a.short,
		Long:    a.long,
		Args:    cobra.ExactArgs(1),
		Example: fmt.Sprintf("  tc job %s Falcon_Build", a.use),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}
			if err := client.SetBuildTypePaused(args[0], a.paused); err != nil {
				return fmt.Errorf("failed to %s job: %w", a.use, err)
			}
			output.Success("%s job %s", a.verb, args[0])
			return nil
		},
	}
}

func newJobPauseCmd() *cobra.Command  { return newJobStateCmd(jobStateActions["pause"]) }
func newJobResumeCmd() *cobra.Command { return newJobStateCmd(jobStateActions["resume"]) }
