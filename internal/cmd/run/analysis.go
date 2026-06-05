package run

import (
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/dustin/go-humanize/english"
	"github.com/spf13/cobra"
)

type runChangesOptions struct {
	noFiles bool
	json    bool
}

func newRunChangesCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runChangesOptions{}

	cmd := &cobra.Command{
		Use:   "changes <id>",
		Short: "Show VCS changes",
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity run changes 12345
  teamcity run changes 12345 --no-files
  teamcity run changes 12345 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunChanges(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.noFiles, "no-files", false, "Hide file list, show commits only")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runRunChanges(f *cmdutil.Factory, runID string, opts *runChangesOptions) error {
	p := f.Printer
	client, err := f.Client()
	if err != nil {
		return err
	}

	changes, err := client.GetBuildChanges(f.Context(), runID)
	if err != nil {
		return fmt.Errorf("failed to get changes: %w", err)
	}

	if opts.json {
		return p.PrintJSON(changes)
	}

	if changes.Count == 0 {
		p.Info("No changes in this run")
		return nil
	}

	_, _ = fmt.Fprintf(p.Out, "CHANGES (%d %s)\n\n", changes.Count, english.PluralWord(changes.Count, "commit", "commits"))

	var firstSHA, lastSHA string
	for i, c := range changes.Change {
		if i == 0 {
			lastSHA = c.Version
		}
		firstSHA = c.Version

		sha := c.Version
		if len(sha) > 7 {
			sha = sha[:7]
		}

		date := ""
		if c.Date != "" {
			if t, err := api.ParseTeamCityTime(c.Date); err == nil {
				date = output.RelativeTime(t)
			}
		}

		_, _ = fmt.Fprintf(p.Out, "%s  %s  %s\n", output.Yellow(sha), output.Faint(c.Username), output.Faint(date))

		comment := strings.TrimSpace(c.Comment)
		if idx := strings.Index(comment, "\n"); idx > 0 {
			comment = comment[:idx]
		}
		_, _ = fmt.Fprintf(p.Out, "  %s\n", comment)

		if !opts.noFiles && c.Files != nil && len(c.Files.File) > 0 {
			for _, af := range c.Files.File {
				changeType := "M"
				switch af.ChangeType {
				case "added":
					changeType = output.Green("A")
				case "removed":
					changeType = output.Red("D")
				case "edited":
					changeType = output.Yellow("M")
				}
				_, _ = fmt.Fprintf(p.Out, "  %s  %s\n", changeType, output.Faint(af.File))
			}
		}
		_, _ = fmt.Fprintln(p.Out)
	}

	if firstSHA != "" && lastSHA != "" && firstSHA != lastSHA {
		first := firstSHA
		last := lastSHA
		if len(first) > 7 {
			first = first[:7]
		}
		if len(last) > 7 {
			last = last[:7]
		}
		_, _ = fmt.Fprintf(p.Out, "%s git diff %s^..%s\n", output.Faint("# For full diff:"), first, last)
	}

	return nil
}

type runTestsOptions struct {
	failed bool
	muted  bool
	json   bool
	limit  int
	job    string
	test   string
	web    bool
}

func newRunTestsCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runTestsOptions{}

	cmd := &cobra.Command{
		Use:   "tests [id]",
		Short: "Show test results",
		Long: `Show test results from a run.

You can specify a run ID directly, or use --job to get the latest run's tests.

Pass --test NAME to follow one test across builds instead of a single run:
  --job X --test NAME    that test's history in job X
  --test NAME            that test's history server-wide`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && cmd.Flags().Changed("job") {
				return api.MutuallyExclusive("id", "job")
			}
			// --test is a cross-build query; a single build has no history.
			if len(args) > 0 && cmd.Flags().Changed("test") {
				return api.Validation("a run ID and --test cannot be combined", "use --job JOB --test NAME for a job's history, or --test NAME alone for server-wide")
			}
			return cobra.MaximumNArgs(1)(cmd, args)
		},
		Example: `  teamcity run tests 12345
  teamcity run tests 12345 --failed
  teamcity run tests --job Falcon_Build
  teamcity run tests --job Falcon_Build --test com.acme.FooTest.bar`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var runID string
			if len(args) > 0 {
				runID = args[0]
			}
			// The default-job fallback only supplies a build to inspect; in
			// history mode (--test) the absence of an explicit --job means
			// server-wide, so don't let a linked/default job narrow it.
			if runID == "" && opts.job == "" && opts.test == "" {
				opts.job = f.ResolveDefaultJob("")
			}
			return runRunTests(f, runID, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.failed, "failed", false, "Show only failed tests, excluding muted")
	cmd.Flags().BoolVar(&opts.muted, "muted", false, "Show only muted failed tests")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 0, "Maximum number of items")
	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Use this job's latest")
	cmd.Flags().StringVar(&opts.test, "test", "", "Follow one test across builds (history) instead of a single run")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open the run's tests in browser")
	cmd.MarkFlagsMutuallyExclusive("failed", "muted")
	cmd.MarkFlagsMutuallyExclusive("json", "web")
	cmd.MarkFlagsMutuallyExclusive("test", "web") // history spans builds — no single page

	return cmd
}

func runRunTests(f *cmdutil.Factory, runID string, opts *runTestsOptions) error {
	p := f.Printer
	client, err := f.Client()
	if err != nil {
		return err
	}

	if opts.test != "" {
		return runTestHistory(f, client, opts)
	}

	resolvedID, _, err := resolveRunID(f.Context(), client, runID, opts.job, "")
	if err != nil {
		return err
	}
	runID = resolvedID

	build, err := client.GetBuild(f.Context(), runID)
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	if opts.web {
		cmdutil.OpenURLOrWarn(p, runTestsBrowserURL(build.WebURL, opts))
		return nil
	}

	f.Analytics.Track(analytics.GroupBuild, analytics.EventTestsViewed, map[string]any{
		"filter":      testsFilter(opts),
		"is_from_job": opts.job != "",
	})

	tests, err := client.GetBuildTests(f.Context(), runID, api.BuildTestsOptions{
		FailedOnly: opts.failed,
		MutedOnly:  opts.muted,
		Limit:      opts.limit,
	})
	if err != nil {
		return fmt.Errorf("failed to get tests: %w", err)
	}

	if opts.json {
		return p.PrintJSON(tests)
	}

	if tests.Count == 0 {
		switch {
		case opts.muted:
			p.Success("No muted failed tests in this run")
		case opts.failed:
			p.Success("No failed tests in this run")
		default:
			p.Info("No tests in this run")
		}
		return nil
	}

	for _, t := range tests.TestOccurrence {
		switch t.Status {
		case "FAILURE":
			if t.Muted {
				_, _ = fmt.Fprintf(p.Out, "%s %s\n", output.Faint(output.Sym().Skip), t.Name)
			} else {
				_, _ = fmt.Fprintf(p.Out, "%s %s\n", output.Red(output.Sym().Cross), t.Name)
			}
		case "SUCCESS":
			_, _ = fmt.Fprintf(p.Out, "%s %s\n", output.Green(output.Sym().Check), t.Name)
		default:
			_, _ = fmt.Fprintf(p.Out, "%s %s\n", output.Faint(output.Sym().Neutral), t.Name)
		}
	}

	_, _ = fmt.Fprintf(p.Out, "\nTESTS: %s\n", output.TestCountsSummary(tests))
	_, _ = fmt.Fprintf(p.Out, "\n%s %s\n", output.Faint("View in browser:"), runTestsBrowserURL(build.WebURL, opts))
	return nil
}

// runTestHistory shows one test across builds: scoped to a job (buildType+test) or server-wide (test alone).
func runTestHistory(f *cmdutil.Factory, client api.ClientInterface, opts *runTestsOptions) error {
	p := f.Printer

	q := api.TestOccurrenceQuery{TestName: opts.test, BuildType: opts.job, Limit: opts.limit}
	switch {
	case opts.failed:
		q.Status, q.Muted = "failed", new(false)
	case opts.muted:
		q.Status, q.Muted = "failed", new(true)
	}

	f.Analytics.Track(analytics.GroupBuild, analytics.EventTestsViewed, map[string]any{
		"filter":      testsFilter(opts),
		"is_from_job": opts.job != "",
	})

	tests, err := client.ListTestOccurrences(f.Context(), q)
	if err != nil {
		return fmt.Errorf("failed to get test history: %w", err)
	}

	if opts.json {
		return p.PrintJSON(tests)
	}

	if tests.Count == 0 {
		p.Info("No occurrences found for test %q", opts.test)
		return nil
	}

	_, _ = fmt.Fprintf(p.Out, "%s %s\n\n", output.Faint("TEST:"), opts.test)
	headers := []string{"BUILD", "STATUS", "DURATION", "BRANCH"}
	rows := make([][]string, 0, len(tests.TestOccurrence))
	for _, t := range tests.TestOccurrence {
		build, branch := "", "-"
		if t.Build != nil {
			build = "#" + t.Build.Number
			if t.Build.BranchName != "" {
				branch = t.Build.BranchName
			}
		}
		rows = append(rows, []string{
			build,
			testStatusLabel(t),
			output.FormatDuration(time.Duration(t.Duration) * time.Millisecond),
			branch,
		})
	}
	output.AutoSizeColumns(headers, rows, 2, 3)
	p.PrintTable(headers, rows)
	_, _ = fmt.Fprintf(p.Out, "\nTESTS: %s\n", output.TestCountsSummary(tests))
	return nil
}

func testsFilter(opts *runTestsOptions) string {
	switch {
	case opts.failed:
		return analytics.TestsFilterFailed
	case opts.muted:
		return analytics.TestsFilterMuted
	default:
		return analytics.TestsFilterAll
	}
}

func testStatusLabel(t api.TestOccurrence) string {
	switch t.Status {
	case "SUCCESS":
		return output.Green("PASS")
	case "FAILURE":
		if t.Muted {
			return output.Faint("MUTED")
		}
		return output.Red("FAIL")
	default:
		return output.Faint("IGNORED")
	}
}

func runTestsBrowserURL(webURL string, opts *runTestsOptions) string {
	separator := "?"
	if strings.Contains(webURL, "?") {
		separator = "&"
	}
	link := webURL + separator + "buildTab=tests"
	if opts.failed {
		return link + "&status=failed"
	}
	if opts.muted {
		return link + "&status=muted"
	}
	return link
}
