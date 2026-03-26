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
		Long:  `Watch a run in real-time until it completes.`,
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity run watch 12345
  teamcity run watch 12345 --interval 10
  teamcity run watch 12345 --logs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := doRunWatch(f, args[0], opts)
			if _, ok := errors.AsType[*cmdutil.ExitError](err); ok {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
			}
			return err
		},
	}

	cmd.Flags().IntVarP(&opts.interval, "interval", "i", 5, "Refresh interval in seconds")
	cmd.Flags().BoolVar(&opts.logs, "logs", false, "Stream build logs while watching")
	cmd.Flags().BoolVarP(&opts.quiet, "quiet", "Q", false, "Minimal output, show only state changes and result")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 0, "Timeout duration (e.g., 30m, 1h)")
	cmd.MarkFlagsMutuallyExclusive("quiet", "logs")

	return cmd
}

func doRunWatch(f *cmdutil.Factory, runID string, opts *runWatchOptions) error {
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
		output.Warn("--logs requires a TTY; falling back to standard watch mode")
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
			fmt.Println()
			if !opts.quiet {
				fmt.Println()
				fmt.Println(output.Faint("Interrupted. Run continues in background."))
				fmt.Printf("%s Resume watching: teamcity run watch %s\n", output.Faint("Hint:"), runID)
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

	if opts.quiet {
		fmt.Printf("Watching: %s\n", build.WebURL)
	} else {
		output.Info("Watching run #%s... %s\n", runID, output.Faint("(Ctrl-C to stop watching)"))
	}

	lastState := ""
	lastPercent := 0
	lastOvertimeMin := 0
	var reachedComplete time.Time
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				fmt.Printf("\n%s Timeout exceeded\n", output.Red("✗"))
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

		if opts.quiet {
			if build.State != lastState {
				switch build.State {
				case "queued":
					fmt.Print("Queued")
				case "running":
					fmt.Print("\rRunning")
				}
				lastState = build.State
			}
			if build.State == "running" {
				pct := build.PercentageComplete
				if pct > lastPercent && pct > 0 {
					fmt.Printf("... %d%%", pct)
					lastPercent = pct
					if pct == 100 {
						reachedComplete = time.Now()
					}
				}
				if pct == 100 && !reachedComplete.IsZero() {
					overtimeMin := int(time.Since(reachedComplete).Minutes())
					if overtimeMin > lastOvertimeMin {
						fmt.Printf("... +%dm", overtimeMin)
						lastOvertimeMin = overtimeMin
					}
				}
			}
		} else {
			status := output.Yellow("Running")
			if build.State == "queued" {
				status = output.Faint("Queued")
			}
			progress := ""
			if build.PercentageComplete > 0 {
				progress = fmt.Sprintf(" (%d%%)", build.PercentageComplete)
			}
			fmt.Printf("\r%s %s %d  #%s %s · %s%s    ",
				output.StatusIcon(build.Status, build.State),
				output.Cyan(jobName),
				build.ID,
				build.Number,
				output.Faint(build.WebURL),
				status,
				progress)
		}

		if build.State == "finished" {
			fmt.Println()
			if !opts.quiet {
				fmt.Println()
			}

			return cmdutil.BuildResultError(client, build, !opts.quiet)
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				fmt.Printf("\n%s Timeout exceeded\n", output.Red("✗"))
				return &cmdutil.ExitError{Code: cmdutil.ExitTimeout}
			}
			return nil
		case <-time.After(time.Duration(opts.interval) * time.Second):
		}
	}
}

