package job

import (
	"slices"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

type jobListOptions struct {
	project string
	all     bool
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
	cmd.Flags().BoolVar(&opts.all, "all", false, "Include pipelines")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 30)

	return cmd
}

func (opts *jobListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	pipelineProjectIDs := map[string]bool{}
	if !opts.all && client.SupportsFeature("pipelines") {
		if pipelines, err := client.GetPipelines(api.PipelinesOptions{Limit: 10000}); err == nil {
			for _, p := range pipelines.Pipelines {
				pipelineProjectIDs[p.ID] = true
			}
		}
	}

	limit := opts.Limit
	if len(pipelineProjectIDs) > 0 {
		limit += limit
	}

	fetchFields := fields
	if len(pipelineProjectIDs) > 0 && len(fields) > 0 && !slices.Contains(fields, "projectId") {
		fetchFields = append(slices.Clone(fields), "projectId")
	}

	jobs, err := client.GetBuildTypes(api.BuildTypesOptions{
		Project: opts.project,
		Limit:   limit,
		Fields:  fetchFields,
	})
	if err != nil {
		return nil, err
	}

	if len(pipelineProjectIDs) > 0 {
		filtered := jobs.BuildTypes[:0]
		for _, j := range jobs.BuildTypes {
			if !isPipelineOwned(j.ProjectID, pipelineProjectIDs) {
				filtered = append(filtered, j)
			}
		}
		jobs.BuildTypes = filtered
		jobs.Count = len(filtered)
	}
	if len(jobs.BuildTypes) > opts.Limit {
		jobs.BuildTypes = jobs.BuildTypes[:opts.Limit]
		jobs.Count = opts.Limit
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

func isPipelineOwned(projectID string, pipelineProjectIDs map[string]bool) bool {
	if pipelineProjectIDs[projectID] {
		return true
	}
	for pid := range pipelineProjectIDs {
		if strings.HasPrefix(projectID, pid+"_") {
			return true
		}
	}
	return false
}
