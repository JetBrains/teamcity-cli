package cmd

import (
	"fmt"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/dustin/go-humanize/english"
	"github.com/spf13/cobra"
)

type runChangesOptions struct {
	noFiles bool
	json    bool
}

func newRunChangesCmd() *cobra.Command {
	opts := &runChangesOptions{}

	cmd := &cobra.Command{
		Use:   "changes <run-id>",
		Short: "Show VCS changes in a run",
		Long:  `Show the VCS changes (commits) included in a run.`,
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity run changes 12345
  teamcity run changes 12345 --no-files
  teamcity run changes 12345 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunChanges(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.noFiles, "no-files", false, "Hide file list, show commits only")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runRunChanges(runID string, opts *runChangesOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	changes, err := client.GetBuildChanges(runID)
	if err != nil {
		return fmt.Errorf("failed to get changes: %w", err)
	}

	if opts.json {
		return output.PrintJSON(changes)
	}

	if changes.Count == 0 {
		output.Info("No changes in this run")
		return nil
	}

	fmt.Printf("CHANGES (%d %s)\n\n", changes.Count, english.PluralWord(changes.Count, "commit", "commits"))

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

		fmt.Printf("%s  %s  %s\n", output.Yellow(sha), output.Faint(c.Username), output.Faint(date))

		comment := strings.TrimSpace(c.Comment)
		if idx := strings.Index(comment, "\n"); idx > 0 {
			comment = comment[:idx]
		}
		fmt.Printf("  %s\n", comment)

		if !opts.noFiles && c.Files != nil && len(c.Files.File) > 0 {
			for _, f := range c.Files.File {
				changeType := "M"
				switch f.ChangeType {
				case "added":
					changeType = output.Green("A")
				case "removed":
					changeType = output.Red("D")
				case "edited":
					changeType = output.Yellow("M")
				}
				fmt.Printf("  %s  %s\n", changeType, output.Faint(f.File))
			}
		}
		fmt.Println()
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
		fmt.Printf("%s git diff %s^..%s\n", output.Faint("# For full diff:"), first, last)
	}

	return nil
}

type runTestsOptions struct {
	failed bool
	json   bool
	limit  int
	job    string
}

func newRunTestsCmd() *cobra.Command {
	opts := &runTestsOptions{}

	cmd := &cobra.Command{
		Use:   "tests [run-id]",
		Short: "Show test results for a run",
		Long: `Show test results from a run.

You can specify a run ID directly, or use --job to get the latest run's tests.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && cmd.Flags().Changed("job") {
				return tcerrors.MutuallyExclusive("run-id", "job")
			}
			return cobra.MaximumNArgs(1)(cmd, args)
		},
		Example: `  teamcity run tests 12345
  teamcity run tests 12345 --failed
  teamcity run tests --job Falcon_Build`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var runID string
			if len(args) > 0 {
				runID = args[0]
			}
			return runRunTests(runID, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.failed, "failed", false, "Show only failed tests")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 0, "Maximum number of tests to show")
	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Get tests for latest run of this job")

	return cmd
}

func runRunTests(runID string, opts *runTestsOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if opts.job != "" {
		runs, err := client.GetBuilds(api.BuildsOptions{
			BuildTypeID: opts.job,
			Limit:       1,
		})
		if err != nil {
			return err
		}
		if runs.Count == 0 || len(runs.Builds) == 0 {
			return fmt.Errorf("no runs found for job %s", opts.job)
		}
		runID = fmt.Sprintf("%d", runs.Builds[0].ID)
	} else if runID == "" {
		return fmt.Errorf("run ID required (or use --job to get latest run)")
	}

	build, err := client.GetBuild(runID)
	if err != nil {
		return fmt.Errorf("failed to get build: %w", err)
	}

	tests, err := client.GetBuildTests(runID, opts.failed, opts.limit)
	if err != nil {
		return fmt.Errorf("failed to get tests: %w", err)
	}

	if opts.json {
		return output.PrintJSON(tests)
	}

	if tests.Count == 0 {
		if opts.failed {
			output.Success("No failed tests in this run")
		} else {
			output.Info("No tests in this run")
		}
		return nil
	}

	fmt.Printf("%s %s\n\n", output.Faint("View in browser:"), build.WebURL+"?buildTab=tests")

	var parts []string
	if tests.Passed > 0 {
		parts = append(parts, output.Green(fmt.Sprintf("%d passed", tests.Passed)))
	}
	if tests.Failed > 0 {
		parts = append(parts, output.Red(fmt.Sprintf("%d failed", tests.Failed)))
	}
	if tests.Ignored > 0 {
		parts = append(parts, output.Faint(fmt.Sprintf("%d ignored", tests.Ignored)))
	}
	fmt.Printf("TESTS: %s\n\n", strings.Join(parts, ", "))

	for _, t := range tests.TestOccurrence {
		switch t.Status {
		case "FAILURE":
			fmt.Printf("%s %s\n", output.Red("✗"), t.Name)
		case "SUCCESS":
			fmt.Printf("%s %s\n", output.Green("✓"), t.Name)
		default:
			fmt.Printf("%s %s\n", output.Faint("○"), t.Name)
		}
	}

	return nil
}
