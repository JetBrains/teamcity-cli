package queue

import (
	"strconv"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type queueListOptions struct {
	job string
	cmdutil.ListFlags
	cmdutil.ViewOptions
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
  teamcity queue list --plain --no-header
  teamcity queue list --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Web {
				if err := cmdutil.ValidateLimit(opts.Limit); err != nil {
					return err
				}
			}
			if done, err := opts.EmitListWebURL(f.Printer, config.ResolveServerURL(), "/queue.html"); done {
				return err
			}
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.QueuedBuildFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Filter by job ID")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 30)
	cmdutil.AddWebFlags(cmd, &opts.ViewOptions)

	_ = cmd.RegisterFlagCompletionFunc("job", completion.LinkedJobs())

	return cmd
}

func (opts *queueListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	queue, truncated, err := client.GetBuildQueue(api.QueueOptions{
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
			strconv.Itoa(r.ID),
			r.BuildTypeID,
			branch,
			r.State,
			waitReason,
		})
	}

	return &cmdutil.ListResult{
		JSON:      queue,
		Table:     cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{1, 2, 4}},
		EmptyMsg:  "No runs in queue",
		EmptyTip:  output.TipNoQueue,
		Truncated: truncated,
	}, nil
}
