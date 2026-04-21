package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

type runLogOptions struct {
	job    string
	failed bool
	raw    bool
	web    bool
	json   bool
	tail   int
	follow bool
}

func newRunLogCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runLogOptions{}

	cmd := &cobra.Command{
		Use:   "log [id]",
		Short: "View log",
		Long: `View the log output from a run.

You can specify a run ID directly, or use --job to get the latest run's log.

Use --tail to show the last N log messages via the structured messages API.
Use --follow to stream logs from a running build until it completes.
Output is plain text and pipe-friendly (e.g., teamcity run log -f 123 | grep ERROR).

For a full-screen interactive TUI, use "teamcity run watch --logs" instead.

Pager: / search, n/N next/prev, g/G top/bottom, q quit.
Use --raw to bypass the pager.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && cmd.Flags().Changed("job") {
				return api.MutuallyExclusive("id", "job")
			}
			return cobra.MaximumNArgs(1)(cmd, args)
		},
		Example: `  teamcity run log 12345
  teamcity run log 12345 --tail 50
  teamcity run log 12345 --follow
  teamcity run log 12345 --follow --tail 200
  teamcity run log 12345 --failed
  teamcity run log 12345 --json
  teamcity run log --job Falcon_Build`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var runID string
			if len(args) > 0 {
				runID = args[0]
			}
			return runRunLog(f, runID, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Use this job's latest")
	cmd.Flags().BoolVar(&opts.failed, "failed", false, "Show failure summary (problems and failed tests)")
	cmd.Flags().BoolVar(&opts.raw, "raw", false, "Show raw log without formatting")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().IntVar(&opts.tail, "tail", 0, "Show last N log messages")
	cmd.Flags().BoolVarP(&opts.follow, "follow", "f", false, "Stream log output until completion")

	cmd.MarkFlagsMutuallyExclusive("json", "raw")
	cmd.MarkFlagsMutuallyExclusive("json", "web")
	cmd.MarkFlagsMutuallyExclusive("failed", "tail")
	cmd.MarkFlagsMutuallyExclusive("failed", "follow")
	cmd.MarkFlagsMutuallyExclusive("web", "tail")
	cmd.MarkFlagsMutuallyExclusive("web", "follow")

	return cmd
}

type buildLogJSON struct {
	RunID string `json:"run_id"`
	Log   string `json:"log"`
}

type failureSummaryJSON struct {
	RunID    string                  `json:"run_id"`
	Number   string                  `json:"number"`
	Status   string                  `json:"status"`
	WebURL   string                  `json:"web_url"`
	Problems []api.ProblemOccurrence `json:"problems"`
	Tests    *api.TestOccurrences    `json:"failed_tests,omitempty"`
}

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
	content = output.RestoreAnsi(content)
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

const (
	msgStatusWarning = 2
	msgStatusError   = 4

	defaultFollowTail  = 100
	followPollInterval = 2 * time.Second
	followFetchWindow  = 500
)

func formatMessage(msg api.BuildMessage, raw bool) string {
	text := strings.TrimRight(msg.Text, "\r\n")
	if text == "" {
		return ""
	}

	if raw {
		return text
	}

	indent := ""
	if msg.Level > 1 {
		indent = strings.Repeat("  ", msg.Level-1)
	}

	ts := ""
	if msg.Timestamp != "" {
		if t, err := time.Parse("2006-01-02T15:04:05.000-0700", msg.Timestamp); err == nil {
			ts = fmt.Sprintf("[%s] ", t.Format("15:04:05"))
		} else if t, err := time.Parse("2006-01-02T15:04:05-0700", msg.Timestamp); err == nil {
			ts = fmt.Sprintf("[%s] ", t.Format("15:04:05"))
		}
	}

	line := fmt.Sprintf("%s%s%s", indent, ts, text)

	switch msg.Status {
	case msgStatusError:
		return output.Red(line)
	case msgStatusWarning:
		return output.Yellow(line)
	default:
		if msg.Verbose {
			return output.Faint(line)
		}
		return line
	}
}

func runRunLog(f *cmdutil.Factory, runID string, opts *runLogOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if opts.job != "" {
		state := "finished"
		if opts.follow {
			state = "any"
		}
		runs, err := client.GetBuilds(context.Background(), api.BuildsOptions{
			BuildTypeID: opts.job,
			State:       state,
			Limit:       1,
		})
		if err != nil {
			return err
		}
		if runs.Count == 0 || len(runs.Builds) == 0 {
			return api.Validation(
				fmt.Sprintf("no runs found for job %q", opts.job),
				"Try --all, or verify the job ID with 'teamcity job list'",
			)
		}
		runID = strconv.Itoa(runs.Builds[0].ID)
		if !opts.json {
			f.Printer.Info("Showing log for #%s (%s)", runID, runs.Builds[0].Number)
		}
	} else if runID == "" {
		return errors.New("run ID required (or use --job to get latest run)")
	}

	if opts.web {
		build, err := client.GetBuild(context.Background(), runID)
		if err != nil {
			return err
		}
		return browser.OpenURL(build.WebURL + "?buildTab=buildLog")
	}

	if opts.failed {
		return runLogFailed(f, client, runID, opts.json)
	}

	if opts.follow {
		return runLogFollow(f, client, runID, opts)
	}

	if opts.tail != 0 {
		if opts.tail < 1 {
			return fmt.Errorf("--tail must be a positive number, got %d", opts.tail)
		}
		return runLogTail(f, client, runID, opts)
	}

	return runLogFull(f, client, runID, opts)
}

func runLogFull(f *cmdutil.Factory, client api.ClientInterface, runID string, opts *runLogOptions) error {
	log, err := client.GetBuildLog(context.Background(), runID)
	if err != nil {
		return fmt.Errorf("failed to get run log: %w", err)
	}

	if opts.json {
		return f.Printer.PrintJSON(buildLogJSON{
			RunID: runID,
			Log:   log,
		})
	}

	if log == "" {
		f.Printer.Empty("No log available", output.TipNoLogFor(runID))
		return nil
	}

	output.WithPager(f.Printer.Out, func(w io.Writer) {
		if opts.raw {
			_, _ = fmt.Fprintln(w, log)
			return
		}
		for line := range strings.SplitSeq(log, "\n") {
			formatted := formatLogLine(line)
			if formatted != "" {
				_, _ = fmt.Fprintln(w, formatted)
			}
		}
	})
	return nil
}

func runLogTail(f *cmdutil.Factory, client api.ClientInterface, runID string, opts *runLogOptions) error {
	resp, err := client.GetBuildMessages(context.Background(), runID, api.BuildMessagesOptions{
		Count:     -opts.tail,
		Tail:      true,
		ExpandAll: true,
	})
	if err != nil {
		return fmt.Errorf("failed to get log messages: %w", err)
	}

	return printMessages(f, runID, resp.Messages, opts)
}

func printMessages(f *cmdutil.Factory, runID string, messages []api.BuildMessage, opts *runLogOptions) error {
	if opts.json {
		return f.Printer.PrintJSON(struct {
			RunID    string             `json:"run_id"`
			Messages []api.BuildMessage `json:"messages"`
		}{RunID: runID, Messages: messages})
	}

	if len(messages) == 0 {
		f.Printer.Empty("No log messages available", output.TipNoLogFor(runID))
		return nil
	}

	w := f.Printer.Out
	for _, msg := range messages {
		line := formatMessage(msg, opts.raw)
		if line != "" {
			_, _ = fmt.Fprintln(w, line)
		}
	}
	return nil
}

func runLogFollow(f *cmdutil.Factory, client api.ClientInterface, runID string, opts *runLogOptions) error {
	p := f.Printer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			_, _ = fmt.Fprintln(p.Out)
			if !opts.json {
				_, _ = fmt.Fprintln(p.Out, output.Faint("Interrupted. Run continues in background."))
				p.Tip("Resume: teamcity run log -f %s", runID)
			}
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	initialTail := defaultFollowTail
	if opts.tail > 0 {
		initialTail = opts.tail
	}

	if err := waitForBuildStart(ctx, p, client, runID, opts.json); err != nil {
		return err
	}

	resp, err := client.GetBuildMessages(context.Background(), runID, api.BuildMessagesOptions{
		Count:     -initialTail,
		Tail:      true,
		ExpandAll: true,
	})
	if err != nil {
		return fmt.Errorf("failed to get log messages: %w", err)
	}

	showVerbose := opts.raw

	lastSeenID := 0
	for _, msg := range resp.Messages {
		if msg.ID > lastSeenID {
			lastSeenID = msg.ID
		}
		printFollowMessage(p.Out, msg, showVerbose, opts.raw, opts.json)
	}

	build, err := client.GetBuild(context.Background(), runID)
	if err != nil {
		return err
	}
	if build.State == "finished" {
		return buildFinishedResult(p, client, build, opts.json)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(followPollInterval):
		}

		resp, err := client.GetBuildMessages(context.Background(), runID, api.BuildMessagesOptions{
			Count:     -followFetchWindow,
			Tail:      true,
			ExpandAll: true,
		})
		if err != nil {
			if build, err := client.GetBuild(context.Background(), runID); err == nil && build.State == "finished" {
				return buildFinishedResult(p, client, build, opts.json)
			}
			continue
		}

		if len(resp.Messages) > 0 && resp.Messages[0].ID > lastSeenID+1 {
			allNew := true
			for _, msg := range resp.Messages {
				if msg.ID <= lastSeenID {
					allNew = false
					break
				}
			}
			if allNew && !opts.json {
				_, _ = fmt.Fprintf(p.Out, "%s some log messages may have been skipped\n", output.Faint("..."))
			}
		}

		for _, msg := range resp.Messages {
			if msg.ID <= lastSeenID {
				continue
			}
			lastSeenID = msg.ID
			printFollowMessage(p.Out, msg, showVerbose, opts.raw, opts.json)
		}

		build, err := client.GetBuild(context.Background(), runID)
		if err != nil {
			continue
		}
		if build.State == "finished" {
			finalResp, err := client.GetBuildMessages(context.Background(), runID, api.BuildMessagesOptions{
				Count:     -followFetchWindow,
				Tail:      true,
				ExpandAll: true,
			})
			if err == nil {
				for _, msg := range finalResp.Messages {
					if msg.ID <= lastSeenID {
						continue
					}
					printFollowMessage(p.Out, msg, showVerbose, opts.raw, opts.json)
				}
			}
			return buildFinishedResult(p, client, build, opts.json)
		}
	}
}

func waitForBuildStart(ctx context.Context, p *output.Printer, client api.ClientInterface, runID string, jsonOut bool) error {
	build, err := client.GetBuild(context.Background(), runID)
	if err != nil {
		return err
	}
	if build.State != "queued" {
		return nil
	}

	if !jsonOut {
		p.Info("Build is queued, waiting for it to start...")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(followPollInterval):
		}

		build, err = client.GetBuild(context.Background(), runID)
		if err != nil {
			return err
		}
		if build.State != "queued" {
			if !jsonOut {
				_, _ = fmt.Fprintln(p.Out)
				p.Info("Build started")
			}
			return nil
		}
		if !jsonOut && build.WaitReason != "" {
			p.Progress("\r  %s", build.WaitReason)
		}
	}
}

func printFollowMessage(w io.Writer, msg api.BuildMessage, showVerbose, raw, jsonOut bool) {
	if !showVerbose && msg.Verbose {
		return
	}
	if jsonOut {
		_, _ = fmt.Fprintln(w, messageToJSON(msg))
		return
	}
	line := formatMessage(msg, raw)
	if line != "" {
		_, _ = fmt.Fprintln(w, line)
	}
}

func buildFinishedResult(p *output.Printer, client api.ClientInterface, build *api.Build, jsonOut bool) error {
	if jsonOut {
		switch build.Status {
		case "SUCCESS":
			return nil
		case "FAILURE":
			return &cmdutil.ExitError{Code: cmdutil.ExitFailure}
		default:
			return &cmdutil.ExitError{Code: cmdutil.ExitCancelled}
		}
	}
	_, _ = fmt.Fprintln(p.Out)
	return cmdutil.BuildResultError(context.Background(), p, client, build, true)
}

func messageToJSON(msg api.BuildMessage) string {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Sprintf(`{"id":%d,"error":"marshal failed"}`, msg.ID)
	}
	return string(data)
}

func runLogFailed(f *cmdutil.Factory, client api.ClientInterface, runID string, jsonOut bool) error {
	build, err := client.GetBuild(context.Background(), runID)
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	if jsonOut {
		summary := failureSummaryJSON{
			RunID:  runID,
			Number: build.Number,
			Status: build.Status,
			WebURL: build.WebURL,
		}

		summary.Problems = []api.ProblemOccurrence{}
		if problems, err := client.GetBuildProblems(runID); err == nil && len(problems.ProblemOccurrence) > 0 {
			summary.Problems = problems.ProblemOccurrence
		}
		if build.Status != "SUCCESS" {
			if tests, err := client.GetBuildTests(context.Background(), runID, true, 0); err == nil {
				summary.Tests = tests
			}
		}
		return f.Printer.PrintJSON(summary)
	}

	if build.Status == "SUCCESS" {
		f.Printer.Success("Build %d  #%s succeeded", build.ID, build.Number)
		return nil
	}
	cmdutil.PrintFailureSummary(context.Background(), f.Printer, client, runID, build.Number, build.WebURL, build.StatusText)
	return nil
}
