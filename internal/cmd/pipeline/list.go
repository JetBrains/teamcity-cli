package pipeline

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type pipelineListOptions struct {
	project string
	cmdutil.ListFlags
}

func newPipelineListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &pipelineListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List pipelines",
		Aliases: []string{"ls"},
		Example: `  teamcity pipeline list
  teamcity pipeline list --project MyProject
  teamcity pipeline list --json
  teamcity pipeline list --json=id,name
  teamcity pipeline list --plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.PipelineFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 30)

	return cmd
}

func (opts *pipelineListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	pipelines, err := client.GetPipelines(api.PipelinesOptions{
		Project: opts.project,
		Limit:   opts.Limit,
		Fields:  fields,
	})
	if err != nil {
		return nil, err
	}

	headers := []string{"ID", "NAME", "PROJECT", "JOBS"}
	var rows [][]string

	for _, p := range pipelines.Pipelines {
		projectName := ""
		if p.ParentProject != nil {
			projectName = p.ParentProject.Name
		}
		jobCount := ""
		if p.Jobs != nil {
			jobCount = fmt.Sprintf("%d", p.Jobs.Count)
		}

		rows = append(rows, []string{
			p.ID,
			p.Name,
			projectName,
			jobCount,
		})
	}

	return &cmdutil.ListResult{
		JSON:     pipelines,
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{0, 1, 2}},
		EmptyMsg: "No pipelines found",
	}, nil
}
