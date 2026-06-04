package run

import (
	"fmt"
	"sort"
	"strings"

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
	failed      bool
	newFailures bool
	muted       bool
	status      string
	groupBy     string
	json        bool
	limit       int
	job         string
}

// resolveStatus collapses the status-selecting flags into a single value
// (one of "", "passed", "failed", "ignored", "new").
func (o *runTestsOptions) resolveStatus() string {
	switch {
	case o.status != "":
		return o.status
	case o.failed:
		return "failed"
	case o.newFailures:
		return "new"
	default:
		return ""
	}
}

func newRunTestsCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runTestsOptions{}

	cmd := &cobra.Command{
		Use:   "tests [id]",
		Short: "Show test results",
		Long: `Show test results from a run.

You can specify a run ID directly, or use --job to get the latest run's tests.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && cmd.Flags().Changed("job") {
				return api.MutuallyExclusive("id", "job")
			}
			return cobra.MaximumNArgs(1)(cmd, args)
		},
		Example: `  teamcity run tests 12345
  teamcity run tests 12345 --status failed
  teamcity run tests 12345 --new-failures
  teamcity run tests 12345 --muted
  teamcity run tests --job Falcon_Build`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var runID string
			if len(args) > 0 {
				runID = args[0]
			}
			if runID == "" && opts.job == "" {
				opts.job = f.ResolveDefaultJob("")
			}
			return runRunTests(f, runID, opts)
		},
	}

	cmd.Flags().StringVar(&opts.status, "status", "", "Filter by status: passed, failed, ignored, new")
	cmd.Flags().StringVar(&opts.groupBy, "group-by", "", "Group output by: suite, package, class")
	cmd.Flags().BoolVar(&opts.failed, "failed", false, "Show only failed tests, excluding muted")
	cmd.Flags().BoolVar(&opts.newFailures, "new-failures", false, "Show only new failing tests")
	cmd.Flags().BoolVar(&opts.muted, "muted", false, "Show only muted failed tests")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 0, "Maximum number of items")
	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Use this job's latest")
	cmd.MarkFlagsMutuallyExclusive("status", "failed", "new-failures", "muted")
	cmdutil.DeprecateFlag(cmd, "failed", "status failed", "v2.0.0")

	return cmd
}

func runRunTests(f *cmdutil.Factory, runID string, opts *runTestsOptions) error {
	p := f.Printer
	client, err := f.Client()
	if err != nil {
		return err
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

	status := opts.resolveStatus()

	filter := analytics.TestsFilterAll
	switch {
	case opts.muted:
		filter = analytics.TestsFilterMuted
	case status == "failed":
		filter = analytics.TestsFilterFailed
	case status == "passed":
		filter = analytics.TestsFilterPassed
	case status == "ignored":
		filter = analytics.TestsFilterIgnored
	case status == "new":
		filter = analytics.TestsFilterNew
	}
	groupBy := opts.groupBy
	if groupBy == "" {
		groupBy = "none"
	}
	f.Analytics.Track(analytics.GroupBuild, analytics.EventTestsViewed, map[string]any{
		"filter":      filter,
		"is_from_job": opts.job != "",
		"group_by":    groupBy,
	})

	tests, err := client.GetBuildTests(f.Context(), runID, api.BuildTestsOptions{
		MutedOnly: opts.muted,
		Status:    status,
		GroupBy:   opts.groupBy,
		Limit:     opts.limit,
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
		case status == "failed":
			p.Success("No failed tests in this run")
		case status == "new":
			p.Success("No new failing tests in this run")
		case status == "passed":
			p.Info("No passed tests in this run")
		case status == "ignored":
			p.Info("No ignored tests in this run")
		default:
			p.Info("No tests in this run")
		}
		return nil
	}

	_, _ = fmt.Fprintf(p.Out, "%s %s\n\n", output.Faint("View in browser:"), runTestsBrowserURL(build.WebURL, status, opts.muted))

	var parts []string
	if tests.Passed > 0 {
		parts = append(parts, output.Green(fmt.Sprintf("%d passed", tests.Passed)))
	}
	if tests.Failed > 0 {
		parts = append(parts, output.Red(fmt.Sprintf("%d failed", tests.Failed)))
	}
	if tests.Muted > 0 {
		parts = append(parts, output.Faint(fmt.Sprintf("%d muted", tests.Muted)))
	}
	if tests.Ignored > 0 {
		parts = append(parts, output.Faint(fmt.Sprintf("%d ignored", tests.Ignored)))
	}
	_, _ = fmt.Fprintf(p.Out, "TESTS: %s\n\n", strings.Join(parts, ", "))

	if opts.groupBy != "" {
		renderGroupedTests(p, tests.TestOccurrence, opts.groupBy)
	} else {
		for _, t := range tests.TestOccurrence {
			_, _ = fmt.Fprintln(p.Out, testLine(t))
		}
	}

	return nil
}

// testLine renders a single occurrence: status symbol, name, and NEW/MUTED badges.
func testLine(t api.TestOccurrence) string {
	var symbol string
	switch t.Status {
	case "FAILURE":
		if t.Muted {
			symbol = output.Faint(output.Sym().Skip)
		} else {
			symbol = output.Red(output.Sym().Cross)
		}
	case "SUCCESS":
		symbol = output.Green(output.Sym().Check)
	default:
		symbol = output.Faint(output.Sym().Neutral)
	}

	line := fmt.Sprintf("%s %s", symbol, t.Name)
	if t.NewFailure {
		line += " " + output.Yellow("NEW")
	}
	if t.Muted {
		line += " " + output.Faint("MUTED")
	}
	return line
}

// renderGroupedTests prints occurrences bucketed by suite/package/class, each
// group headed by its key and per-group count, in stable alphabetical order.
func renderGroupedTests(p *output.Printer, occ []api.TestOccurrence, dimension string) {
	groups := make(map[string][]api.TestOccurrence)
	var order []string
	for _, t := range occ {
		key := testGroupKey(t, dimension)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], t)
	}
	sort.Strings(order)

	for i, key := range order {
		if i > 0 {
			_, _ = fmt.Fprintln(p.Out)
		}
		members := groups[key]
		_, _ = fmt.Fprintf(p.Out, "%s %s\n", output.Bold(key), output.Faint(fmt.Sprintf("(%d)", len(members))))
		for _, t := range members {
			_, _ = fmt.Fprintf(p.Out, "  %s\n", testLine(t))
		}
	}
}

// testGroupKey derives the grouping key from the occurrence's parsed test name,
// falling back to "(ungrouped)" when the requested dimension is unavailable.
func testGroupKey(t api.TestOccurrence, dimension string) string {
	if t.Test != nil && t.Test.ParsedTestName != nil {
		p := t.Test.ParsedTestName
		switch dimension {
		case "suite":
			if p.TestSuite != "" {
				return p.TestSuite
			}
		case "package":
			if p.TestPackage != "" {
				return p.TestPackage
			}
		case "class":
			if p.TestClass != "" {
				return p.TestClass
			}
		}
	}
	return "(ungrouped)"
}

func runTestsBrowserURL(webURL, status string, muted bool) string {
	separator := "?"
	if strings.Contains(webURL, "?") {
		separator = "&"
	}
	link := webURL + separator + "buildTab=tests"
	switch {
	case muted:
		return link + "&status=muted"
	case status == "failed", status == "new":
		return link + "&status=failed"
	case status == "passed":
		return link + "&status=success"
	}
	return link
}
