package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

type runListOptions struct {
	job        string
	branch     string
	status     string
	user       string
	project    string
	limit      int
	since      string
	until      string
	jsonFields string
	plain      bool
	noHeader   bool
	web        bool
}

func newRunListCmd() *cobra.Command {
	opts := &runListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent runs",
		Example: `  tc run list
  tc run list --job Sandbox_Demo
  tc run list --status failure --limit 10
  tc run list --project Sandbox --branch main
  tc run list --since 24h
  tc run list --json
  tc run list --json=id,status,webUrl
  tc run list --plain | grep failure`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunList(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Filter by job ID")
	cmd.Flags().StringVarP(&opts.branch, "branch", "b", "", "Filter by branch name")
	cmd.Flags().StringVar(&opts.status, "status", "", "Filter by status (success, failure, running)")
	cmd.Flags().StringVarP(&opts.user, "user", "u", "", "Filter by user who triggered")
	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 30, "Maximum number of runs")
	cmd.Flags().StringVar(&opts.since, "since", "", "Filter builds finished after this time (e.g., 24h, 2026-01-21)")
	cmd.Flags().StringVar(&opts.until, "until", "", "Filter builds finished before this time (e.g., 12h, 2026-01-22)")
	AddJSONFieldsFlag(cmd, &opts.jsonFields)
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "Output in plain text format for scripting")
	cmd.Flags().BoolVar(&opts.noHeader, "no-header", false, "Omit header row (use with --plain)")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")

	cmd.MarkFlagsMutuallyExclusive("json", "plain")

	return cmd
}

func runRunList(cmd *cobra.Command, opts *runListOptions) error {
	jsonResult, showHelp, err := ParseJSONFields(cmd, opts.jsonFields, &api.BuildFields)
	if err != nil {
		return err
	}
	if showHelp {
		return nil
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	if opts.web {
		url := config.GetServerURL() + "/builds"
		return browser.OpenURL(url)
	}

	user := opts.user
	if user == "@me" {
		user = config.GetCurrentUser()
		if user == "" {
			return fmt.Errorf("@me requires login (username not found in config)")
		}
	}

	// Validate status if provided
	if opts.status != "" {
		validStatuses := []string{"success", "failure", "running", "error", "unknown"}
		status := strings.ToLower(opts.status)
		valid := false
		for _, v := range validStatuses {
			if status == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid status %q, must be one of: success, failure, running", opts.status)
		}
	}

	var sinceDate, untilDate string
	if opts.since != "" {
		sinceDate, err = api.ParseUserDate(opts.since)
		if err != nil {
			return fmt.Errorf("invalid --since date: %w", err)
		}
	}
	if opts.until != "" {
		untilDate, err = api.ParseUserDate(opts.until)
		if err != nil {
			return fmt.Errorf("invalid --until date: %w", err)
		}
	}

	runs, err := client.GetBuilds(api.BuildsOptions{
		BuildTypeID: opts.job,
		Branch:      opts.branch,
		Status:      opts.status,
		User:        user,
		Project:     opts.project,
		Limit:       opts.limit,
		SinceDate:   sinceDate,
		UntilDate:   untilDate,
		Fields:      jsonResult.Fields,
	})
	if err != nil {
		return err
	}

	if jsonResult.Enabled {
		return output.PrintJSON(runs)
	}

	if runs.Count == 0 {
		output.Info("No runs found")
		return nil
	}

	var headers []string
	if opts.plain {
		headers = []string{"STATUS", "ID", "JOB", "BRANCH", "TRIGGERED_BY", "DURATION", "AGE"}
	} else {
		headers = []string{"STATUS", "RUN", "JOB", "BRANCH", "TRIGGERED BY", "DURATION", "AGE"}
	}
	var rows [][]string

	widths := output.ColumnWidths(47, 30, 40, 35, 25)

	maybeTruncate := func(s string, maxLen int) string {
		if opts.plain {
			return s
		}
		return output.Truncate(s, maxLen)
	}

	for _, r := range runs.Builds {
		var status, runRef string
		if opts.plain {
			status = output.PlainStatusText(r.Status, r.State)
			runRef = fmt.Sprintf("%d", r.ID)
		} else {
			status = fmt.Sprintf("%s %s", output.StatusIcon(r.Status, r.State), output.StatusText(r.Status, r.State))
			runRef = fmt.Sprintf("#%s (%d)", r.Number, r.ID)
		}

		triggeredBy := "-"
		if r.Triggered != nil && r.Triggered.User != nil {
			triggeredBy = r.Triggered.User.Name
		} else if r.Triggered != nil {
			triggeredBy = r.Triggered.Type
		}

		duration := "-"
		age := "-"

		if r.StartDate != "" {
			startTime, _ := api.ParseTeamCityTime(r.StartDate)
			if r.FinishDate != "" {
				finishTime, _ := api.ParseTeamCityTime(r.FinishDate)
				duration = output.FormatDuration(finishTime.Sub(startTime))
				age = output.RelativeTime(finishTime)
			} else {
				duration = output.FormatDuration(time.Since(startTime))
				age = "now"
			}
		} else if r.QueuedDate != "" {
			queuedTime, _ := api.ParseTeamCityTime(r.QueuedDate)
			age = output.RelativeTime(queuedTime)
		}

		branch := r.BranchName
		if branch == "" {
			branch = "-"
		}

		rows = append(rows, []string{
			status,
			runRef,
			maybeTruncate(r.BuildTypeID, widths[0]),
			maybeTruncate(branch, widths[1]),
			maybeTruncate(triggeredBy, widths[2]),
			duration,
			age,
		})
	}

	if opts.plain {
		output.PrintPlainTable(headers, rows, opts.noHeader)
	} else {
		output.PrintTable(headers, rows)
	}
	return nil
}

type runViewOptions struct {
	json bool
	web  bool
}

func newRunViewCmd() *cobra.Command {
	opts := &runViewOptions{}

	cmd := &cobra.Command{
		Use:   "view <run-id>",
		Short: "View run details",
		Args:  cobra.ExactArgs(1),
		Example: `  tc run view 12345
  tc run view 12345 --web
  tc run view 12345 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunView(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")

	return cmd
}

func runRunView(runID string, opts *runViewOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	build, err := client.GetBuild(runID)
	if err != nil {
		return err
	}

	if opts.web {
		return browser.OpenURL(build.WebURL)
	}

	if opts.json {
		return output.PrintJSON(build)
	}

	icon := output.StatusIcon(build.Status, build.State)
	jobName := build.BuildTypeID
	if build.BuildType != nil {
		jobName = build.BuildType.Name
	}

	fmt.Printf("%s %s #%s", icon, output.Cyan(jobName), build.Number)
	if build.BranchName != "" {
		fmt.Printf(" · %s", build.BranchName)
	}
	fmt.Println()

	if build.Triggered != nil {
		triggeredBy := build.Triggered.Type
		if build.Triggered.User != nil {
			triggeredBy = build.Triggered.User.Name
		}
		fmt.Printf("Triggered by %s", triggeredBy)

		if build.StartDate != "" {
			startTime, _ := api.ParseTeamCityTime(build.StartDate)
			fmt.Printf(" · %s", output.RelativeTime(startTime))

			if build.FinishDate != "" {
				finishTime, _ := api.ParseTeamCityTime(build.FinishDate)
				duration := finishTime.Sub(startTime)
				fmt.Printf(" · Took %s", output.FormatDuration(duration))
			}
		}
		fmt.Println()
	}

	if build.StatusText != "" && build.StatusText != build.Status {
		fmt.Printf("\nStatus: %s\n", build.StatusText)
	}

	if build.State == "running" && build.PercentageComplete > 0 {
		fmt.Printf("\nProgress: %d%%\n", build.PercentageComplete)
	}

	if build.Agent != nil {
		fmt.Printf("\nAgent: %s\n", output.Faint(build.Agent.Name))
	}

	if build.Pinned {
		fmt.Printf("\n%s\n", output.Yellow("📌 Pinned"))
	}

	if build.Tags != nil && len(build.Tags.Tag) > 0 {
		var tagNames []string
		for _, t := range build.Tags.Tag {
			tagNames = append(tagNames, t.Name)
		}
		fmt.Printf("\nTags: %s\n", strings.Join(tagNames, ", "))
	}

	fmt.Printf("\n%s %s\n", output.Faint("View in browser:"), output.Green(build.WebURL))

	return nil
}
