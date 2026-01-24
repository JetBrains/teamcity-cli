package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/api"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

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
		if runs.Count == 0 || len(runs.Builds) == 0 {
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
