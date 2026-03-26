package run

import (
	"fmt"
	"io"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type runLogOptions struct {
	job    string
	failed bool
	raw    bool
}

func newRunLogCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runLogOptions{}

	cmd := &cobra.Command{
		Use:   "log [run-id]",
		Short: "View run log",
		Long: `View the log output from a run.

You can specify a run ID directly, or use --job to get the latest run's log.

Pager: / search, n/N next/prev, g/G top/bottom, q quit.
Use --raw to bypass the pager.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && cmd.Flags().Changed("job") {
				return tcerrors.MutuallyExclusive("run-id", "job")
			}
			return cobra.MaximumNArgs(1)(cmd, args)
		},
		Example: `  teamcity run log 12345
  teamcity run log 12345 --failed
  teamcity run log --job Falcon_Build`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var runID string
			if len(args) > 0 {
				runID = args[0]
			}
			return runRunLog(f, runID, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Get log for latest run of this job")
	cmd.Flags().BoolVar(&opts.failed, "failed", false, "Show failure summary (problems and failed tests)")
	cmd.Flags().BoolVar(&opts.raw, "raw", false, "Show raw log without formatting")

	return cmd
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

func runRunLog(f *cmdutil.Factory, runID string, opts *runLogOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if opts.job != "" {
		runs, err := client.GetBuilds(api.BuildsOptions{
			BuildTypeID: opts.job,
			State:       "finished",
			Limit:       1,
		})
		if err != nil {
			return err
		}
		if runs.Count == 0 || len(runs.Builds) == 0 {
			return fmt.Errorf("no runs found for job %s", opts.job)
		}
		runID = fmt.Sprintf("%d", runs.Builds[0].ID)
		output.Info("Showing log for run %s  #%s", runID, runs.Builds[0].Number)
	} else if runID == "" {
		return fmt.Errorf("run ID required (or use --job to get latest run)")
	}

	if opts.failed {
		build, err := client.GetBuild(runID)
		if err != nil {
			return fmt.Errorf("failed to get build: %w", err)
		}
		if build.Status == "SUCCESS" {
			output.Success("Build %d  #%s succeeded", build.ID, build.Number)
			return nil
		}
		cmdutil.PrintFailureSummary(client, runID, build.Number, build.WebURL, build.StatusText)
		return nil
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

	output.WithPager(func(w io.Writer) {
		if opts.raw {
			_, _ = fmt.Fprintln(w, log)
		} else {
			for _, line := range lines {
				formatted := formatLogLine(line)
				if formatted != "" {
					_, _ = fmt.Fprintln(w, formatted)
				}
			}
		}
	})
	return nil
}
