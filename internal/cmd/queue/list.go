package queue

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type queueListOptions struct {
	job        string
	limit      int
	jsonFields string
}

func newQueueListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &queueListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List queued runs",
		Long:    `List all runs in the TeamCity queue.`,
		Aliases: []string{"ls"},
		Example: `  teamcity queue list
  teamcity queue list --job Falcon_Build
  teamcity queue list --json
  teamcity queue list --json=id,state,webUrl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueueList(f, cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Filter by job ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 30, "Maximum number of queued runs")
	cmdutil.AddJSONFieldsFlag(cmd, &opts.jsonFields)

	return cmd
}

func runQueueList(f *cmdutil.Factory, cmd *cobra.Command, opts *queueListOptions) error {
	if err := cmdutil.ValidateLimit(opts.limit); err != nil {
		return err
	}
	jsonResult, showHelp, err := cmdutil.ParseJSONFields(cmd, opts.jsonFields, &api.QueuedBuildFields)
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

	queue, err := client.GetBuildQueue(api.QueueOptions{
		BuildTypeID: opts.job,
		Limit:       opts.limit,
		Fields:      jsonResult.Fields,
	})
	if err != nil {
		return err
	}

	if jsonResult.Enabled {
		return output.PrintJSON(queue)
	}

	if queue.Count == 0 {
		fmt.Println("No runs in queue")
		return nil
	}

	headers := []string{"ID", "JOB", "BRANCH", "STATE"}
	var rows [][]string

	for _, r := range queue.Builds {
		branch := r.BranchName
		if branch == "" {
			branch = "<default>"
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", r.ID),
			r.BuildTypeID,
			branch,
			r.State,
		})
	}

	output.AutoSizeColumns(headers, rows, 2, 1, 2)
	output.PrintTable(headers, rows)
	return nil
}
