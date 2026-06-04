package test

import (
	"fmt"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type historyOptions struct {
	project string
	job     string
	json    bool
	cmdutil.ListFlags
}

func newHistoryCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &historyOptions{}

	cmd := &cobra.Command{
		Use:   "history <test>",
		Short: "Show a test's pass/fail timeline across builds",
		Long: `Show the pass/fail timeline of a single test within a project or job.

A scope is required — pass --project or --job. The footer reports the pass rate
and average duration over the runs shown. Use --json for the raw occurrence
array (suitable for flakiness analysis).`,
		Example: `  teamcity test history com.example.FooTest.shouldWork --project Falcon
  teamcity test history com.example.FooTest.shouldWork --job Falcon_Build
  teamcity test history com.example.FooTest.shouldWork --project Falcon --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistory(f, cmd, opts, args[0])
		},
	}

	cmd.Flags().StringVar(&opts.project, "project", "", "Project to query across all its builds")
	cmd.Flags().StringVar(&opts.job, "job", "", "Job to query (takes precedence over --project)")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output the raw test-occurrence array as JSON")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "n", 50, "Maximum number of runs (0 for all)")
	cmdutil.AddPlainFlags(cmd, &opts.ListFlags)

	return cmd
}

func runHistory(f *cmdutil.Factory, cmd *cobra.Command, opts *historyOptions, name string) error {
	p := f.Printer

	if err := cmdutil.ValidateLimit(opts.Limit); err != nil {
		return err
	}

	project := f.ResolveProject(opts.project)
	job := f.ResolveDefaultJob(opts.job)

	// An explicit --project without --job means project-wide; don't let a linked job override it.
	if cmd.Flags().Changed("project") && !cmd.Flags().Changed("job") {
		job = ""
	}

	if project == "" && job == "" {
		return api.Validation(
			"a scope is required for test history",
			"pass --project or --job (server-wide queries are not allowed)",
		)
	}

	f.Analytics.Track(analytics.GroupTest, analytics.EventTestHistoryViewed, map[string]any{
		"is_from_job": job != "",
	})

	client, err := f.Client()
	if err != nil {
		return err
	}

	occ, err := client.GetTestHistory(f.Context(), name, api.TestQueryOptions{
		Project: project,
		Job:     job,
		Limit:   opts.Limit,
	})
	if err != nil {
		return err
	}

	if opts.json {
		return p.PrintJSON(occ)
	}

	if occ.Count == 0 || len(occ.TestOccurrence) == 0 {
		p.Empty(fmt.Sprintf("No runs of %q found in this scope", name), output.TipNoTests)
		return nil
	}

	headers := []string{"BUILD", "STATUS", "DURATION", "BRANCH", "WHEN"}
	rows := make([][]string, 0, len(occ.TestOccurrence))
	for _, t := range occ.TestOccurrence {
		rows = append(rows, []string{
			historyBuild(t),
			historyStatus(t),
			historyDuration(t),
			historyBranch(t),
			historyWhen(t),
		})
	}

	if opts.Plain {
		p.PrintPlainTable(headers, rows, opts.NoHeader)
	} else {
		output.AutoSizeColumns(headers, rows, 2, 3)
		p.PrintTable(headers, rows)
		_, _ = fmt.Fprintf(p.Out, "\n%s\n", historyFooter(computeTestStats(occ.TestOccurrence)))
	}

	return nil
}

func historyBuild(t api.TestOccurrence) string {
	if t.Build == nil || t.Build.Number == "" {
		return "-"
	}
	return "#" + t.Build.Number
}

func historyStatus(t api.TestOccurrence) string {
	switch t.Status {
	case "SUCCESS":
		return output.Green("pass")
	case "FAILURE":
		if t.Muted {
			return output.Faint("fail (muted)")
		}
		return output.Red("fail")
	default:
		return output.Faint("ignored")
	}
}

func historyDuration(t api.TestOccurrence) string {
	if t.Duration <= 0 {
		return "-"
	}
	return output.FormatDuration(time.Duration(t.Duration) * time.Millisecond)
}

func historyBranch(t api.TestOccurrence) string {
	if t.Build == nil || t.Build.BranchName == "" {
		return "-"
	}
	return t.Build.BranchName
}

func historyWhen(t api.TestOccurrence) string {
	if t.Build == nil || t.Build.StartDate == "" {
		return "-"
	}
	ts, err := api.ParseTeamCityTime(t.Build.StartDate)
	if err != nil {
		return "-"
	}
	return output.RelativeTime(ts)
}

// testStats summarizes a test's pass/fail timeline. Pass-rate and average
// duration are computed over runs that actually ran (passed + failed), so
// ignored runs neither help nor hurt.
type testStats struct {
	Passed      int
	Failed      int
	Ignored     int
	Total       int
	Considered  int
	PassRate    float64
	AvgDuration time.Duration
}

func computeTestStats(occ []api.TestOccurrence) testStats {
	var s testStats
	var totalDur time.Duration
	for _, t := range occ {
		s.Total++
		switch t.Status {
		case "SUCCESS":
			s.Passed++
			totalDur += time.Duration(t.Duration) * time.Millisecond
		case "FAILURE":
			s.Failed++
			totalDur += time.Duration(t.Duration) * time.Millisecond
		default:
			s.Ignored++
		}
	}
	s.Considered = s.Passed + s.Failed
	if s.Considered > 0 {
		s.PassRate = float64(s.Passed) / float64(s.Considered) * 100
		s.AvgDuration = totalDur / time.Duration(s.Considered)
	}
	return s
}

func historyFooter(s testStats) string {
	rate := fmt.Sprintf("Pass rate: %.0f%% (%d/%d)", s.PassRate, s.Passed, s.Considered)
	if s.Considered == 0 {
		rate = "Pass rate: n/a (0 runs)"
	}
	avg := "Avg duration: -"
	if s.Considered > 0 {
		avg = "Avg duration: " + output.FormatDuration(s.AvgDuration)
	}
	return output.Faint(rate + "  |  " + avg)
}
