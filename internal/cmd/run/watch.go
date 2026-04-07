package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/cmd/run/tui"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type runWatchOptions struct {
	interval int
	logs     bool
	quiet    bool
	json     bool
	timeout  time.Duration
}

var runWatchTUIFn = tui.RunWatchTUI
var watchHasTTYFn = func() bool {
	return output.IsTerminal() && output.IsStdinTerminal()
}

func newRunWatchCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runWatchOptions{}

	cmd := &cobra.Command{
		Use:   "watch <run-id>",
		Short: "Watch a run until it completes",
		Long: `Watch a run in real-time until it completes.

Shows build status with periodic polling. Use --logs for a full-screen TUI
with live log output.

For a simpler, pipe-friendly log stream, use "teamcity run log --follow" instead.`,
		Args: cobra.ExactArgs(1),
		Example: `  teamcity run watch 12345
  teamcity run watch 12345 --interval 10
  teamcity run watch 12345 --logs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return doRunWatch(f, args[0], opts)
		},
	}

	cmd.Flags().IntVarP(&opts.interval, "interval", "i", 5, "Refresh interval in seconds")
	cmd.Flags().BoolVar(&opts.logs, "logs", false, "Stream build logs while watching")
	cmd.Flags().BoolVar(&opts.quiet, "quiet", false, "Minimal output, show only state changes and result")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Wait for completion and output result as JSON")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 0, "Timeout duration (e.g., 30m, 1h)")
	cmd.MarkFlagsMutuallyExclusive("quiet", "logs")
	cmd.MarkFlagsMutuallyExclusive("json", "logs")
	cmd.MarkFlagsMutuallyExclusive("json", "quiet")

	return cmd
}

func doRunWatch(f *cmdutil.Factory, runID string, opts *runWatchOptions) error {
	p := f.Printer
	if f.Quiet {
		opts.quiet = true
	}
	if opts.interval < 1 {
		return fmt.Errorf("--interval must be at least 1 second, got %d", opts.interval)
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	if opts.logs && !opts.quiet {
		if watchHasTTYFn() {
			return runWatchTUIFn(client, runID, opts.interval)
		}
		p.Warn("--logs requires a TTY; falling back to standard watch mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if opts.timeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, opts.timeout)
		defer timeoutCancel()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			if !opts.json {
				_, _ = fmt.Fprintln(p.Out)
			}
			if !opts.quiet && !opts.json {
				_, _ = fmt.Fprintln(p.Out)
				_, _ = fmt.Fprintln(p.Out, output.Faint("Interrupted. Run continues in background."))
				_, _ = fmt.Fprintf(p.Out, "%s Resume watching: teamcity run watch %s\n", output.Faint("Hint:"), runID)
			}
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	build, err := client.GetBuild(runID)
	if err != nil {
		return err
	}

	switch {
	case opts.json:
		// silent until completion
	case opts.quiet:
		_, _ = fmt.Fprintf(p.Out, "Watching: %s\n", build.WebURL)
	default:
		p.Info("Watching run #%s... %s\n", runID, output.Faint("(Ctrl-C to stop watching)"))
	}

	lastState := ""
	lastWaitReason := ""
	lastPercent := 0
	lastOvertimeMin := 0
	var reachedComplete time.Time
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				if !opts.json {
					_, _ = fmt.Fprintf(p.Out, "\n%s Timeout exceeded\n", output.Red("✗"))
				}
				return &cmdutil.ExitError{Code: cmdutil.ExitTimeout}
			}
			return nil
		default:
		}

		build, err = client.GetBuild(runID)
		if err != nil {
			return err
		}

		jobName := build.BuildTypeID
		if build.BuildType != nil {
			jobName = build.BuildType.Name
		}

		switch {
		case opts.json:
			// silent polling — no output until completion
		case opts.quiet:
			if build.State != lastState {
				switch build.State {
				case "queued":
					_, _ = fmt.Fprint(p.Out, "Queued")
				case "running":
					_, _ = fmt.Fprint(p.Out, "\rRunning")
				}
				lastState = build.State
			}
			if build.State == "queued" && build.WaitReason != "" && build.WaitReason != lastWaitReason {
				_, _ = fmt.Fprintf(p.Out, "\rQueued (%s)", build.WaitReason)
				lastWaitReason = build.WaitReason
			}
			if build.State == "running" {
				pct := build.PercentageComplete
				if pct > lastPercent && pct > 0 {
					_, _ = fmt.Fprintf(p.Out, "... %d%%", pct)
					lastPercent = pct
					if pct == 100 {
						reachedComplete = time.Now()
					}
				}
				if pct == 100 && !reachedComplete.IsZero() {
					overtimeMin := int(time.Since(reachedComplete).Minutes())
					if overtimeMin > lastOvertimeMin {
						_, _ = fmt.Fprintf(p.Out, "... +%dm", overtimeMin)
						lastOvertimeMin = overtimeMin
					}
				}
			}
		default:
			status := output.Yellow("Running")
			if build.State == "queued" {
				status = output.Faint("Queued")
				if build.WaitReason != "" {
					status = output.Faint(fmt.Sprintf("Queued · %s", build.WaitReason))
				}
			}
			progress := ""
			if build.PercentageComplete > 0 {
				progress = fmt.Sprintf(" (%d%%)", build.PercentageComplete)
			}
			_, _ = fmt.Fprintf(p.Out, "\r%s %s %d  #%s %s · %s%s    ",
				output.StatusIcon(build.Status, build.State),
				output.Cyan(jobName),
				build.ID,
				build.Number,
				output.Faint(build.WebURL),
				status,
				progress)
		}

		if build.State == "finished" {
			if opts.json {
				if err := p.PrintJSON(build); err != nil {
					return err
				}
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
			if !opts.quiet {
				_, _ = fmt.Fprintln(p.Out)
			}

			return cmdutil.BuildResultError(p, client, build, !opts.quiet)
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				if !opts.json {
					_, _ = fmt.Fprintf(p.Out, "\n%s Timeout exceeded\n", output.Red("✗"))
				}
				return &cmdutil.ExitError{Code: cmdutil.ExitTimeout}
			}
			return nil
		case <-time.After(time.Duration(opts.interval) * time.Second):
		}
	}
}
