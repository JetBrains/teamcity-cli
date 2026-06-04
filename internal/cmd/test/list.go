package test

import (
	"cmp"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type listOptions struct {
	project      string
	job          string
	failing      bool
	muted        bool
	investigated bool
	cmdutil.ListFlags
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List currently failing, muted, or investigated tests across builds",
		Aliases: []string{"ls"},
		Long: `List tests across all builds in a project or job.

A scope is required — pass --project or --job. Server-wide queries are rejected.
By default lists currently failing tests; use --muted or --investigated instead.`,
		Example: `  teamcity test list --project Falcon
  teamcity test list --project Falcon --muted
  teamcity test list --job Falcon_Build --investigated
  teamcity test list --project Falcon --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(f, cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.project, "project", "", "Project to query across all its builds")
	cmd.Flags().StringVar(&opts.job, "job", "", "Job to query (takes precedence over --project)")
	cmd.Flags().BoolVar(&opts.failing, "failing", false, "Currently failing tests (default)")
	cmd.Flags().BoolVar(&opts.muted, "muted", false, "Currently muted tests")
	cmd.Flags().BoolVar(&opts.investigated, "investigated", false, "Currently investigated tests")
	cmd.MarkFlagsMutuallyExclusive("failing", "muted", "investigated")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 100)

	return cmd
}

func runList(f *cmdutil.Factory, cmd *cobra.Command, opts *listOptions) error {
	project := f.ResolveProject(opts.project)
	job := f.ResolveDefaultJob(opts.job)

	// An explicit --project without --job means project-wide; don't let a linked job override it.
	if cmd.Flags().Changed("project") && !cmd.Flags().Changed("job") {
		job = ""
	}

	if project == "" && job == "" {
		return api.Validation(
			"a scope is required for cross-build test queries",
			"pass --project or --job (server-wide queries are not allowed)",
		)
	}

	f.Analytics.Track(analytics.GroupTest, analytics.EventTestListed, map[string]any{
		"filter":      listFilter(opts),
		"is_from_job": job != "",
	})

	return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.TestListFields, func(client api.ClientInterface, _ []string) (*cmdutil.ListResult, error) {
		occ, err := client.ListTests(f.Context(), api.TestQueryOptions{
			Project:      project,
			Job:          job,
			Failing:      opts.failing,
			Muted:        opts.muted,
			Investigated: opts.investigated,
			Limit:        opts.Limit,
		})
		if err != nil {
			return nil, err
		}

		headers := []string{"TEST", "JOB", "SINCE"}
		var rows [][]string
		for _, t := range occ.TestOccurrence {
			rows = append(rows, []string{t.Name, occurrenceJob(t), occurrenceSince(t)})
		}

		return &cmdutil.ListResult{
			JSON:     occ,
			Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{0, 1}},
			EmptyMsg: emptyMsg(opts),
			EmptyTip: output.TipNoTests,
		}, nil
	})
}

func occurrenceJob(t api.TestOccurrence) string {
	if t.Build == nil || t.Build.BuildType == nil {
		return "-"
	}
	return cmp.Or(t.Build.BuildType.Name, t.Build.BuildType.ID)
}

// occurrenceSince renders when/where the test currently shows: relative time of the
// build start when available, always tagged with the build number.
func occurrenceSince(t api.TestOccurrence) string {
	if t.Build == nil {
		return "-"
	}
	num := "#" + t.Build.Number
	if t.Build.StartDate != "" {
		if ts, err := api.ParseTeamCityTime(t.Build.StartDate); err == nil {
			return output.RelativeTime(ts) + " " + output.Faint(num)
		}
	}
	return num
}

func listFilter(opts *listOptions) string {
	switch {
	case opts.muted:
		return analytics.TestFilterMuted
	case opts.investigated:
		return analytics.TestFilterInvestigated
	default:
		return analytics.TestFilterFailing
	}
}

func emptyMsg(opts *listOptions) string {
	switch {
	case opts.muted:
		return "No muted tests in this scope"
	case opts.investigated:
		return "No investigated tests in this scope"
	default:
		return "No failing tests in this scope"
	}
}
