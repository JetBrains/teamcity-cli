package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/api"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/spf13/cobra"
)

type runArtifactsOptions struct {
	job  string
	json bool
}

func newRunArtifactsCmd() *cobra.Command {
	opts := &runArtifactsOptions{}

	cmd := &cobra.Command{
		Use:   "artifacts [run-id]",
		Short: "List run artifacts",
		Long: `List artifacts from a run without downloading them.

Shows artifact names and sizes. Use tc run download to download artifacts.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && cmd.Flags().Changed("job") {
				return tcerrors.MutuallyExclusive("run-id", "job")
			}
			return cobra.MaximumNArgs(1)(cmd, args)
		},
		Example: `  tc run artifacts 12345
  tc run artifacts 12345 --json
  tc run artifacts --job MyBuild`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var runID string
			if len(args) > 0 {
				runID = args[0]
			}
			return runRunArtifacts(runID, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "List artifacts from latest run of this job")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runRunArtifacts(runID string, opts *runArtifactsOptions) error {
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
		output.Info("Listing artifacts for run #%s (ID: %s)", runs.Builds[0].Number, runID)
	} else if runID == "" {
		return fmt.Errorf("run ID required (or use --job to get latest run)")
	}

	artifacts, err := client.GetArtifacts(runID)
	if err != nil {
		return fmt.Errorf("failed to get artifacts: %w", err)
	}

	if opts.json {
		return output.PrintJSON(artifacts)
	}

	if artifacts.Count == 0 {
		output.Info("No artifacts found for this run")
		return nil
	}

	flatList, totalSize := flattenArtifacts(artifacts.File, "")

	nameWidth := 4 // "NAME"
	for _, a := range flatList {
		if len(a.Name) > nameWidth {
			nameWidth = len(a.Name)
		}
	}

	fmt.Printf("ARTIFACTS (%d %s, %s total)\n\n", len(flatList), english.PluralWord(len(flatList), "file", "files"), humanize.IBytes(uint64(totalSize)))
	fmt.Printf("%-*s  %10s\n", nameWidth, "NAME", "SIZE")

	for _, a := range flatList {
		size := ""
		if a.Size > 0 {
			size = humanize.IBytes(uint64(a.Size))
		}
		fmt.Printf("%-*s  %s\n", nameWidth, a.Name, output.Faint(fmt.Sprintf("%10s", size)))
	}

	fmt.Printf("\nDownload all: tc run download %s\n", runID)
	fmt.Printf("Download one: tc run download %s -a \"<name>\"\n", runID)
	return nil
}

func flattenArtifacts(artifacts []api.Artifact, prefix string) ([]api.Artifact, int64) {
	var result []api.Artifact
	var totalSize int64
	for _, a := range artifacts {
		name := a.Name
		if prefix != "" {
			name = prefix + "/" + a.Name
		}
		if a.Children != nil && len(a.Children.File) > 0 {
			nested, size := flattenArtifacts(a.Children.File, name)
			result = append(result, nested...)
			totalSize += size
		} else {
			result = append(result, api.Artifact{Name: name, Size: a.Size})
			totalSize += a.Size
		}
	}
	return result, totalSize
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

	flatList, totalSize := flattenArtifacts(artifacts.File, "")

	if opts.artifact != "" {
		if _, err := filepath.Match(opts.artifact, ""); err != nil {
			return fmt.Errorf("invalid artifact pattern %q: %w", opts.artifact, err)
		}
		var filtered []api.Artifact
		var filteredSize int64
		for _, a := range flatList {
			if matched, _ := filepath.Match(opts.artifact, filepath.Base(a.Name)); matched {
				filtered = append(filtered, a)
				filteredSize += a.Size
			}
		}
		flatList = filtered
		totalSize = filteredSize
	}

	if len(flatList) == 0 {
		fmt.Println("No artifacts match the pattern")
		return nil
	}

	nameWidth := len("NAME")
	for _, a := range flatList {
		if len(a.Name) > nameWidth {
			nameWidth = len(a.Name)
		}
	}

	fmt.Printf("Downloading %d %s (%s total) to %s\n\n",
		len(flatList), english.PluralWord(len(flatList), "file", "files"),
		humanize.IBytes(uint64(totalSize)), opts.dir)
	fmt.Printf("%-*s  %10s\n", nameWidth, "NAME", "SIZE")

	ctx := context.Background()
	downloaded := 0
	for _, artifact := range flatList {
		outputPath := filepath.Join(opts.dir, artifact.Name)
		size := humanize.IBytes(uint64(artifact.Size))

		if err := downloadArtifact(ctx, client, runID, artifact, outputPath, nameWidth); err != nil {
			fmt.Printf("%-*s  %10s  %s %v\n", nameWidth, artifact.Name, size, output.Red("   ✗"), err)
			continue
		}
		fmt.Printf("%-*s  %10s  %s\n", nameWidth, artifact.Name, size, output.Green("   ✓"))
		downloaded++
	}

	fmt.Printf("\n%s %s downloaded\n", output.Green("✓"), english.Plural(downloaded, "artifact", ""))
	return nil
}

func downloadArtifact(ctx context.Context, client api.ClientInterface, runID string, artifact api.Artifact, outputPath string, nameWidth int) error {
	if dir := filepath.Dir(outputPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}

	var w io.Writer = f
	if output.IsTerminal() && !output.Quiet && artifact.Size > 0 {
		pw := &progressWriter{
			w:         f,
			name:      artifact.Name,
			size:      humanize.IBytes(uint64(artifact.Size)),
			total:     artifact.Size,
			nameWidth: nameWidth,
		}
		w = pw
		defer pw.clear()
	}

	written, err := client.DownloadArtifactTo(ctx, runID, artifact.Name, w)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(outputPath)
		return err
	}

	if artifact.Size > 0 && written != artifact.Size {
		_ = f.Close()
		_ = os.Remove(outputPath)
		return fmt.Errorf("incomplete: got %d/%d bytes", written, artifact.Size)
	}

	return f.Close()
}

type progressWriter struct {
	w          io.Writer
	name       string
	size       string
	total      int64
	written    int64
	nameWidth  int
	lastUpdate time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.written += int64(n)

	now := time.Now()
	if now.Sub(p.lastUpdate) >= 100*time.Millisecond {
		p.lastUpdate = now
		pct := int(float64(p.written) / float64(p.total) * 100)
		_, _ = fmt.Fprintf(os.Stdout, "\r%-*s  %10s  %3d%%", p.nameWidth, p.name, p.size, pct)
	}
	return n, err
}

func (p *progressWriter) clear() {
	_, _ = fmt.Fprint(os.Stdout, "\r\033[K") // Clear line
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

You can specify a run ID directly, or use --job to get the latest run's log.

Pager: / search, n/N next/prev, g/G top/bottom, q quit.
Use --raw to bypass the pager.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && cmd.Flags().Changed("job") {
				return tcerrors.MutuallyExclusive("run-id", "job")
			}
			return cobra.MaximumNArgs(1)(cmd, args)
		},
		Example: `  tc run log 12345
  tc run log 12345 --failed
  tc run log --job Falcon_Build`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var runID string
			if len(args) > 0 {
				runID = args[0]
			}
			return runRunLog(runID, opts)
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

	if opts.failed {
		build, err := client.GetBuild(runID)
		if err != nil {
			return fmt.Errorf("failed to get build: %w", err)
		}
		if build.Status == "SUCCESS" {
			output.Success("Build #%s succeeded", build.Number)
			return nil
		}
		printFailureSummary(client, runID, build.Number, build.WebURL, build.StatusText)
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
