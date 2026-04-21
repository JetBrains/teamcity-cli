package queue

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type queueListOptions struct {
	job string
	cmdutil.ListFlags
}

func newQueueListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &queueListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List queued runs",
		Aliases: []string{"ls"},
		Example: `  teamcity queue list
  teamcity queue list --job Falcon_Build
  teamcity queue list --json
  teamcity queue list --json=id,state,webUrl
  teamcity queue list --plain
  teamcity queue list --plain --no-header`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.QueuedBuildFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Filter by job ID")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 30)

	return cmd
}

func (opts *queueListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	queue, err := client.GetBuildQueue(api.QueueOptions{
		BuildTypeID: opts.job,
		Limit:       opts.Limit,
		Fields:      fields,
	})
	if err != nil {
		return nil, err
	}

	headers := []string{"ID", "JOB", "BRANCH", "STATE", "WAIT REASON"}
	var rows [][]string

	for _, r := range queue.Builds {
		branch := r.BranchName
		if branch == "" {
			branch = "<default>"
		}

		waitReason := r.WaitReason
		if waitReason == "" {
			waitReason = "-"
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", r.ID),
			r.BuildTypeID,
			branch,
			r.State,
			waitReason,
		})
	}

	return &cmdutil.ListResult{
		JSON:     queue,
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{1, 2, 4}},
		EmptyMsg: "No runs in queue",
		EmptyTip: output.TipNoQueue,
	}, nil
}
