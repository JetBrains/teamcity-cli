package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dustin/go-humanize"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/api"
	"github.com/tiulpin/teamcity-cli/internal/config"
	tcerrors "github.com/tiulpin/teamcity-cli/internal/errors"
	"github.com/tiulpin/teamcity-cli/internal/output"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Manage runs (builds)",
		Long:  `List, view, start, and manage TeamCity runs (builds).`,
	}

	cmd.AddCommand(newRunListCmd())
	cmd.AddCommand(newRunViewCmd())
	cmd.AddCommand(newRunStartCmd())
	cmd.AddCommand(newRunCancelCmd())
	cmd.AddCommand(newRunWatchCmd())
	cmd.AddCommand(newRunRestartCmd())
	cmd.AddCommand(newRunDownloadCmd())
	cmd.AddCommand(newRunLogCmd())
	cmd.AddCommand(newRunPinCmd())
	cmd.AddCommand(newRunUnpinCmd())
	cmd.AddCommand(newRunTagCmd())
	cmd.AddCommand(newRunUntagCmd())
	cmd.AddCommand(newRunCommentCmd())
	cmd.AddCommand(newRunChangesCmd())
	cmd.AddCommand(newRunTestsCmd())

	return cmd
}

type runListOptions struct {
	job      string
	branch   string
	status   string
	user     string
	project  string
	limit    int
	json     bool
	plain    bool
	noHeader bool
	web      bool
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
  tc run list --plain | grep failure`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunList(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Filter by job ID")
	cmd.Flags().StringVarP(&opts.branch, "branch", "b", "", "Filter by branch name")
	cmd.Flags().StringVar(&opts.status, "status", "", "Filter by status (success, failure, running)")
	cmd.Flags().StringVarP(&opts.user, "user", "u", "", "Filter by user who triggered")
	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 30, "Maximum number of runs")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "Output in plain text format for scripting")
	cmd.Flags().BoolVar(&opts.noHeader, "no-header", false, "Omit header row (use with --plain)")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")

	cmd.MarkFlagsMutuallyExclusive("json", "plain")

	return cmd
}

func runRunList(opts *runListOptions) error {
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

	runs, err := client.GetBuilds(api.BuildsOptions{
		BuildTypeID: opts.job,
		Branch:      opts.branch,
		Status:      opts.status,
		User:        user,
		Project:     opts.project,
		Limit:       opts.limit,
	})
	if err != nil {
		return err
	}

	if opts.json {
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
			output.Truncate(r.BuildTypeID, widths[0]),
			output.Truncate(branch, widths[1]),
			output.Truncate(triggeredBy, widths[2]),
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

	fmt.Printf("\n%s %s\n", output.Faint("View in browser:"), output.Green(build.WebURL))

	return nil
}

type runStartOptions struct {
	branch            string
	params            map[string]string
	systemProps       map[string]string
	envVars           map[string]string
	comment           string
	personal          bool
	cleanSources      bool
	rebuildDeps       bool
	rebuildFailedDeps bool
	queueAtTop        bool
	agent             int
	tags              []string
	watch             bool
	web               bool
	dryRun            bool
	json              bool
}

func newRunStartCmd() *cobra.Command {
	opts := &runStartOptions{
		params:      make(map[string]string),
		systemProps: make(map[string]string),
		envVars:     make(map[string]string),
	}

	cmd := &cobra.Command{
		Use:   "start <job-id>",
		Short: "Start a new run",
		Args:  cobra.ExactArgs(1),
		Example: `  tc run start Sandbox_Demo
  tc run start Sandbox_Demo --branch feature/test
  tc run start Sandbox_Demo -P version=1.0 -S build.number=123 -E CI=true
  tc run start Sandbox_Demo --comment "Release build" --tag release --tag v1.0
  tc run start Sandbox_Demo --clean --rebuild-deps --top
  tc run start Sandbox_Demo --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunStart(args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.branch, "branch", "b", "", "Branch to build")
	cmd.Flags().StringToStringVarP(&opts.params, "param", "P", nil, "Build parameters (key=value)")
	cmd.Flags().StringToStringVarP(&opts.systemProps, "system", "S", nil, "System properties (key=value)")
	cmd.Flags().StringToStringVarP(&opts.envVars, "env", "E", nil, "Environment variables (key=value)")
	cmd.Flags().StringVarP(&opts.comment, "comment", "m", "", "Run comment")
	cmd.Flags().StringSliceVarP(&opts.tags, "tag", "t", nil, "Run tags (can be repeated)")
	cmd.Flags().BoolVar(&opts.personal, "personal", false, "Run as personal build")
	cmd.Flags().BoolVar(&opts.cleanSources, "clean", false, "Clean sources before run")
	cmd.Flags().BoolVar(&opts.rebuildDeps, "rebuild-deps", false, "Rebuild all dependencies")
	cmd.Flags().BoolVar(&opts.rebuildFailedDeps, "rebuild-failed-deps", false, "Rebuild failed/incomplete dependencies")
	cmd.Flags().BoolVar(&opts.queueAtTop, "top", false, "Add to top of queue")
	cmd.Flags().IntVar(&opts.agent, "agent", 0, "Run on specific agent (by ID)")
	cmd.Flags().BoolVar(&opts.watch, "watch", false, "Watch run until it completes")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open run in browser")
	cmd.Flags().BoolVarP(&opts.dryRun, "dry-run", "n", false, "Show what would be triggered without running")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON (for scripting)")

	return cmd
}

func runRunStart(jobID string, opts *runStartOptions) error {
	if opts.dryRun {
		fmt.Printf("%s Would trigger run for %s\n", output.Faint("[dry-run]"), output.Cyan(jobID))
		if opts.branch != "" {
			fmt.Printf("  Branch: %s\n", opts.branch)
		}
		if len(opts.params) > 0 {
			fmt.Println("  Parameters:")
			for k, v := range opts.params {
				fmt.Printf("    %s=%s\n", k, v)
			}
		}
		if len(opts.systemProps) > 0 {
			fmt.Println("  System properties:")
			for k, v := range opts.systemProps {
				fmt.Printf("    %s=%s\n", k, v)
			}
		}
		if len(opts.envVars) > 0 {
			fmt.Println("  Environment variables:")
			for k, v := range opts.envVars {
				fmt.Printf("    %s=%s\n", k, v)
			}
		}
		if opts.comment != "" {
			fmt.Printf("  Comment: %s\n", opts.comment)
		}
		if len(opts.tags) > 0 {
			fmt.Printf("  Tags: %s\n", strings.Join(opts.tags, ", "))
		}
		if opts.personal {
			fmt.Println("  Personal build: yes")
		}
		if opts.cleanSources {
			fmt.Println("  Clean sources: yes")
		}
		if opts.rebuildDeps {
			fmt.Println("  Rebuild dependencies: yes")
		}
		if opts.queueAtTop {
			fmt.Println("  Queue at top: yes")
		}
		if opts.agent > 0 {
			fmt.Printf("  Agent ID: %d\n", opts.agent)
		}
		return nil
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	build, err := client.RunBuild(jobID, api.RunBuildOptions{
		Branch:                    opts.branch,
		Params:                    opts.params,
		SystemProps:               opts.systemProps,
		EnvVars:                   opts.envVars,
		Comment:                   opts.comment,
		Personal:                  opts.personal,
		CleanSources:              opts.cleanSources,
		RebuildDependencies:       opts.rebuildDeps,
		RebuildFailedDependencies: opts.rebuildFailedDeps,
		QueueAtTop:                opts.queueAtTop,
		AgentID:                   opts.agent,
		Tags:                      opts.tags,
	})
	if err != nil {
		return err
	}

	if opts.json {
		return output.PrintJSON(build)
	}

	// Build number is assigned when the build starts, not when queued
	runRef := fmt.Sprintf("#%s", build.Number)
	if build.Number == "" {
		runRef = fmt.Sprintf("(ID: %d)", build.ID)
	}
	output.Success("Queued run %s for %s", runRef, jobID)

	if opts.branch != "" {
		output.Info("  Branch: %s", opts.branch)
	}
	if opts.comment != "" {
		output.Info("  Comment: %s", opts.comment)
	}
	if len(opts.tags) > 0 {
		output.Info("  Tags: %s", strings.Join(opts.tags, ", "))
	}

	output.Info("  URL: %s", build.WebURL)

	if opts.web {
		browser.OpenURL(build.WebURL)
	}

	if opts.watch {
		fmt.Println()
		return doRunWatch(fmt.Sprintf("%d", build.ID), &runWatchOptions{interval: 5})
	}

	return nil
}

type runCancelOptions struct {
	comment string
	force   bool
}

func newRunCancelCmd() *cobra.Command {
	opts := &runCancelOptions{}

	cmd := &cobra.Command{
		Use:   "cancel <run-id>",
		Short: "Cancel a running build",
		Long:  `Cancel a running or queued run.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc run cancel 12345
  tc run cancel 12345 --comment "Cancelling for hotfix"
  tc run cancel 12345 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunCancel(args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.comment, "comment", "", "Comment for cancellation")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runRunCancel(runID string, opts *runCancelOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	needsConfirmation := !opts.force && opts.comment == "" && !NoInput && output.IsStdinTerminal()

	if needsConfirmation {
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Cancel run #%s?", runID),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			output.Info("Cancelled")
			return nil
		}
	}

	comment := opts.comment
	if comment == "" {
		comment = "Cancelled via tc CLI"
	}

	if err := client.CancelBuild(runID, comment); err != nil {
		return err
	}

	output.Success("Cancelled run #%s", runID)
	return nil
}

type runWatchOptions struct {
	interval int
}

func newRunWatchCmd() *cobra.Command {
	opts := &runWatchOptions{}

	cmd := &cobra.Command{
		Use:   "watch <run-id>",
		Short: "Watch a run until it completes",
		Long:  `Watch a run in real-time until it completes.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc run watch 12345
  tc run watch 12345 --interval 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return doRunWatch(args[0], opts)
		},
	}

	cmd.Flags().IntVarP(&opts.interval, "interval", "i", 5, "Refresh interval in seconds")

	return cmd
}

func doRunWatch(runID string, opts *runWatchOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		fmt.Println()
		fmt.Println()
		fmt.Println(output.Faint("Interrupted. Run continues in background."))
		fmt.Printf("%s Resume watching: tc run watch %s\n", output.Faint("Hint:"), runID)
		cancel()
	}()

	output.Info("Watching run #%s... %s\n", runID, output.Faint("(Ctrl-C to stop watching)"))

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		build, err := client.GetBuild(runID)
		if err != nil {
			return err
		}

		jobName := build.BuildTypeID
		if build.BuildType != nil {
			jobName = build.BuildType.Name
		}

		status := output.Yellow("Running")
		if build.State == "queued" {
			status = output.Faint("Queued")
		}

		progress := ""
		if build.PercentageComplete > 0 {
			progress = fmt.Sprintf(" (%d%%)", build.PercentageComplete)
		}

		fmt.Printf("\r%s %s #%s %s%s    ", output.StatusIcon(build.Status, build.State), output.Cyan(jobName), build.Number, status, progress)

		if build.State == "finished" {
			fmt.Println()
			fmt.Println()

			if build.Status == "SUCCESS" {
				fmt.Printf("%s Run succeeded!\n", output.Green("✓"))
			} else {
				fmt.Printf("%s Run failed: %s\n", output.Red("✗"), build.StatusText)
			}

			fmt.Printf("\nView details: %s\n", build.WebURL)
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Duration(opts.interval) * time.Second):
		}
	}
}

type runRestartOptions struct {
	watch bool
	web   bool
}

func newRunRestartCmd() *cobra.Command {
	opts := &runRestartOptions{}

	cmd := &cobra.Command{
		Use:   "restart <run-id>",
		Short: "Restart a run",
		Long:  `Restart a run with the same configuration.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc run restart 12345
  tc run restart 12345 --watch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunRestart(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.watch, "watch", false, "Watch the new run after restarting")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open run in browser")

	return cmd
}

func runRunRestart(runID string, opts *runRestartOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	originalBuild, err := client.GetBuild(runID)
	if err != nil {
		return fmt.Errorf("failed to get run: %w", err)
	}

	newBuild, err := client.RunBuild(originalBuild.BuildTypeID, api.RunBuildOptions{
		Branch: originalBuild.BranchName,
	})
	if err != nil {
		return fmt.Errorf("failed to trigger run: %w", err)
	}

	// Build number is assigned when the build starts, not when queued
	newRef := fmt.Sprintf("#%s", newBuild.Number)
	if newBuild.Number == "" {
		newRef = fmt.Sprintf("(ID: %d)", newBuild.ID)
	}
	fmt.Printf("%s Queued run %s (restart of #%s)\n", output.Green("✓"), newRef, originalBuild.Number)
	fmt.Printf("  Job: %s\n", originalBuild.BuildTypeID)
	if originalBuild.BranchName != "" {
		fmt.Printf("  Branch: %s\n", originalBuild.BranchName)
	}
	fmt.Printf("  URL: %s\n", newBuild.WebURL)

	if opts.web {
		browser.OpenURL(newBuild.WebURL)
	}

	if opts.watch {
		fmt.Println()
		return doRunWatch(fmt.Sprintf("%d", newBuild.ID), &runWatchOptions{interval: 5})
	}

	return nil
}

type runDownloadOptions struct {
	dir      string
	artifact string
}

func newRunDownloadCmd() *cobra.Command {
	opts := &runDownloadOptions{}

	cmd := &cobra.Command{
		Use:   "download <run-id>",
		Short: "Download run artifacts",
		Long:  `Download artifacts from a completed run.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc run download 12345
  tc run download 12345 --dir ./artifacts
  tc run download 12345 --artifact "*.jar"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunDownload(args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.dir, "dir", "d", ".", "Directory to download artifacts to")
	cmd.Flags().StringVarP(&opts.artifact, "artifact", "a", "", "Artifact name pattern to download")

	return cmd
}

func runRunDownload(runID string, opts *runDownloadOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(opts.dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	artifacts, err := client.GetArtifacts(runID)
	if err != nil {
		return fmt.Errorf("failed to get artifacts: %w", err)
	}

	if artifacts.Count == 0 {
		fmt.Println("No artifacts found for this run")
		return nil
	}

	downloaded := 0
	for _, artifact := range artifacts.File {
		if opts.artifact != "" {
			matched, _ := filepath.Match(opts.artifact, artifact.Name)
			if !matched {
				continue
			}
		}

		fmt.Printf("Downloading %s %s\n", artifact.Name, output.Faint(formatSize(artifact.Size)))

		data, err := client.DownloadArtifact(runID, artifact.Name)
		if err != nil {
			fmt.Printf("  Failed: %v\n", err)
			continue
		}

		outputPath := filepath.Join(opts.dir, artifact.Name)
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			fmt.Printf("  Failed to write: %v\n", err)
			continue
		}

		downloaded++
	}

	fmt.Printf("\n%s Downloaded %d artifact(s) to %s\n", output.Green("✓"), downloaded, opts.dir)
	return nil
}

func formatSize(bytes int64) string {
	if bytes == 0 {
		return ""
	}
	return fmt.Sprintf("(%s)", humanize.IBytes(uint64(bytes)))
}

type runLogOptions struct {
	job    string
	failed bool
	raw    bool
}

func newRunLogCmd() *cobra.Command {
	opts := &runLogOptions{}

	cmd := &cobra.Command{
		Use:   "log [run-id]",
		Short: "View run log",
		Long: `View the log output from a run.

You can specify a run ID directly, or use --job to get the latest run's log.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && cmd.Flags().Changed("job") {
				return tcerrors.MutuallyExclusive("run-id", "job")
			}
			return cobra.MaximumNArgs(1)(cmd, args)
		},
		Example: `  tc run log 12345
  tc run log 12345 --failed
  tc run log --job Sandbox_Demo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var runID string
			if len(args) > 0 {
				runID = args[0]
			}
			return runRunLog(runID, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Get log for latest run of this job")
	cmd.Flags().BoolVar(&opts.failed, "failed", false, "Show only failed step logs")
	cmd.Flags().BoolVar(&opts.raw, "raw", false, "Show raw log without formatting")

	return cmd
}

// formatLogLine formats a TeamCity log line for better readability
// Input: [HH:MM:SS]X: message where X is i(info), e(error), w(warning), or space(normal)
func formatLogLine(line string) string {
	line = strings.TrimSuffix(line, "\r")
	if strings.TrimSpace(line) == "" {
		return ""
	}

	if len(line) < 12 || line[0] != '[' {
		return "  " + line
	}

	closeBracket := strings.Index(line, "]")
	if closeBracket == -1 || closeBracket < 9 {
		return line
	}

	timestamp := line[1:closeBracket]
	rest := line[closeBracket+1:]

	msgType := " "
	content := rest
	if len(rest) >= 2 && rest[1] == ':' {
		msgType = string(rest[0])
		content = rest[2:]
	} else if len(rest) >= 3 && rest[0] == ' ' && rest[1] == ':' {
		content = rest[2:]
	}
	content = strings.TrimPrefix(content, " ")

	switch msgType {
	case "i":
		return output.Faint(fmt.Sprintf("[%s] %s", timestamp, content))
	case "e", "E":
		return output.Red(fmt.Sprintf("[%s] %s", timestamp, content))
	case "w", "W":
		return output.Yellow(fmt.Sprintf("[%s] %s", timestamp, content))
	default:
		return fmt.Sprintf("[%s] %s", timestamp, content)
	}
}

func runRunLog(runID string, opts *runLogOptions) error {
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
		if runs.Count == 0 {
			return fmt.Errorf("no runs found for job %s", opts.job)
		}
		runID = fmt.Sprintf("%d", runs.Builds[0].ID)
		output.Info("Showing log for run #%s (ID: %s)", runs.Builds[0].Number, runID)
	} else if runID == "" {
		return fmt.Errorf("run ID required (or use --job to get latest run)")
	}

	log, err := client.GetBuildLog(runID)
	if err != nil {
		return fmt.Errorf("failed to get run log: %w", err)
	}

	if log == "" {
		output.Info("No log available for this run")
		return nil
	}

	lines := strings.Split(log, "\n")

	if opts.failed {
		var errorLines []string
		inErrorSection := false

		for _, line := range lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "failure") {
				inErrorSection = true
			}
			if inErrorSection {
				errorLines = append(errorLines, line)
				if len(errorLines) > 50 {
					break
				}
			}
		}

		if len(errorLines) > 0 {
			for _, line := range errorLines {
				if opts.raw {
					fmt.Println(line)
				} else {
					fmt.Println(formatLogLine(line))
				}
			}
			return nil
		}
	}

	output.WithPager(func(w io.Writer) {
		if opts.raw {
			fmt.Fprintln(w, log)
		} else {
			for _, line := range lines {
				formatted := formatLogLine(line)
				if formatted != "" {
					fmt.Fprintln(w, formatted)
				}
			}
		}
	})
	return nil
}

type runPinOptions struct {
	comment string
}

func newRunPinCmd() *cobra.Command {
	opts := &runPinOptions{}

	cmd := &cobra.Command{
		Use:   "pin <run-id>",
		Short: "Pin a run to prevent cleanup",
		Long:  `Pin a run to prevent it from being automatically cleaned up by retention policies.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc run pin 12345
  tc run pin 12345 --comment "Release candidate"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunPin(args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.comment, "comment", "m", "", "Comment explaining why the run is pinned")

	return cmd
}

func runRunPin(runID string, opts *runPinOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if err := client.PinBuild(runID, opts.comment); err != nil {
		return fmt.Errorf("failed to pin run: %w", err)
	}

	output.Success("Pinned run #%s", runID)
	if opts.comment != "" {
		output.Info("  Comment: %s", opts.comment)
	}
	return nil
}

func newRunUnpinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unpin <run-id>",
		Short:   "Unpin a run",
		Long:    `Remove the pin from a run, allowing it to be cleaned up by retention policies.`,
		Args:    cobra.ExactArgs(1),
		Example: `  tc run unpin 12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunUnpin(args[0])
		},
	}

	return cmd
}

func runRunUnpin(runID string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if err := client.UnpinBuild(runID); err != nil {
		return fmt.Errorf("failed to unpin run: %w", err)
	}

	output.Success("Unpinned run #%s", runID)
	return nil
}

func newRunTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag <run-id> <tag>...",
		Short: "Add tags to a run",
		Long:  `Add one or more tags to a run for categorization and filtering.`,
		Args:  cobra.MinimumNArgs(2),
		Example: `  tc run tag 12345 release
  tc run tag 12345 release v1.0 production`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunTag(args[0], args[1:])
		},
	}

	return cmd
}

func runRunTag(runID string, tags []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if err := client.AddBuildTags(runID, tags); err != nil {
		return fmt.Errorf("failed to add tags: %w", err)
	}

	output.Success("Added %d tag(s) to run #%s", len(tags), runID)
	output.Info("  Tags: %s", strings.Join(tags, ", "))
	return nil
}

func newRunUntagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "untag <run-id> <tag>...",
		Short: "Remove tags from a run",
		Long:  `Remove one or more tags from a run.`,
		Args:  cobra.MinimumNArgs(2),
		Example: `  tc run untag 12345 release
  tc run untag 12345 release v1.0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunUntag(args[0], args[1:])
		},
	}

	return cmd
}

func runRunUntag(runID string, tags []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	var errors []string
	removed := 0
	for _, tag := range tags {
		if err := client.RemoveBuildTag(runID, tag); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", tag, err))
		} else {
			removed++
		}
	}

	if removed > 0 {
		output.Success("Removed %d tag(s) from run #%s", removed, runID)
	}

	if len(errors) > 0 {
		for _, e := range errors {
			output.Warn("  Failed: %s", e)
		}
		if removed == 0 {
			return fmt.Errorf("failed to remove any tags")
		}
	}

	return nil
}

type runCommentOptions struct {
	delete bool
}

func newRunCommentCmd() *cobra.Command {
	opts := &runCommentOptions{}

	cmd := &cobra.Command{
		Use:   "comment <run-id> [comment]",
		Short: "Set or view run comment",
		Long: `Set, view, or delete a comment on a run.

Without a comment argument, displays the current comment.
With a comment argument, sets the comment.
Use --delete to remove the comment.`,
		Args: cobra.RangeArgs(1, 2),
		Example: `  tc run comment 12345
  tc run comment 12345 "Deployed to production"
  tc run comment 12345 --delete`,
		RunE: func(cmd *cobra.Command, args []string) error {
			comment := ""
			if len(args) > 1 {
				comment = args[1]
			}
			return runRunComment(args[0], comment, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.delete, "delete", false, "Delete the comment")

	return cmd
}

func runRunComment(runID string, comment string, opts *runCommentOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if opts.delete {
		if err := client.DeleteBuildComment(runID); err != nil {
			return fmt.Errorf("failed to delete comment: %w", err)
		}
		output.Success("Deleted comment from run #%s", runID)
		return nil
	}

	if comment != "" {
		if err := client.SetBuildComment(runID, comment); err != nil {
			return fmt.Errorf("failed to set comment: %w", err)
		}
		output.Success("Set comment on run #%s", runID)
		output.Info("  Comment: %s", comment)
		return nil
	}

	existingComment, err := client.GetBuildComment(runID)
	if err != nil {
		return fmt.Errorf("failed to get comment: %w", err)
	}

	if existingComment == "" {
		output.Info("No comment set on run #%s", runID)
	} else {
		fmt.Println(existingComment)
	}
	return nil
}

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
		Example: `  tc run changes 12345
  tc run changes 12345 --no-files
  tc run changes 12345 --json`,
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

	fmt.Printf("CHANGES (%d commit%s)\n\n", changes.Count, pluralize(changes.Count))

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

func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
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
		Example: `  tc run tests 12345
  tc run tests 12345 --failed
  tc run tests --job Sandbox_Demo`,
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
		if runs.Count == 0 {
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
		output.Info("No tests in this run")
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
