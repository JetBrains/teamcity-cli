package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/spf13/cobra"
)

type runDownloadOptions struct {
	output   string
	path     string
	artifact string
	timeout  time.Duration
}

func newRunDownloadCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runDownloadOptions{}

	cmd := &cobra.Command{
		Use:   "download <id>",
		Short: "Download artifacts",
		Long: `Download artifacts from a completed run.

Filter by --artifact (glob) and --path (subdirectory within the run's
artifact tree). Use --output to choose the local destination directory
(defaults to the current directory).`,
		Args: cobra.ExactArgs(1),
		Example: `  teamcity run download 12345
  teamcity run download 12345 --path build/assets
  teamcity run download 12345 -o ./artifacts
  teamcity run download 12345 --artifact "*.jar"
  teamcity run download 12345 --path build/assets -a "*.js"
  teamcity run download 12345 --timeout 30m`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunDownload(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.output, "output", "o", ".", "Local directory to save artifacts to")
	cmd.Flags().StringVarP(&opts.path, "path", "p", "", "Download artifacts under this subdirectory")
	cmd.Flags().StringVarP(&opts.artifact, "artifact", "a", "", "Artifact name pattern to filter")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 10*time.Minute, "Download timeout (e.g. 30m, 1h)")

	_ = cmd.MarkFlagDirname("output")

	return cmd
}

func runRunDownload(f *cmdutil.Factory, runID string, opts *runDownloadOptions) error {
	p := f.Printer
	client, err := f.Client()
	if err != nil {
		return err
	}

	absOutput, err := filepath.Abs(opts.output)
	if err != nil {
		return fmt.Errorf("failed to resolve output path: %w", err)
	}

	if err := os.MkdirAll(absOutput, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(f.Context(), opts.timeout)
	defer cancel()

	flatList, totalSize, err := fetchAllArtifacts(ctx, client, runID, opts.path)
	if err != nil {
		return fmt.Errorf("failed to get artifacts: %w", err)
	}

	if len(flatList) == 0 {
		if opts.path != "" {
			_, _ = fmt.Fprintf(p.Out, "No artifacts found under %s\n", opts.path)
		} else {
			_, _ = fmt.Fprintln(p.Out, "No artifacts found for this run")
		}
		return nil
	}

	if opts.artifact != "" {
		flatList, totalSize, err = filterArtifacts(flatList, opts.artifact)
		if err != nil {
			return err
		}
	}

	if len(flatList) == 0 {
		_, _ = fmt.Fprintln(p.Out, "No artifacts match the pattern")
		return nil
	}

	nameWidth := len("NAME")
	for _, a := range flatList {
		if len(a.Name) > nameWidth {
			nameWidth = len(a.Name)
		}
	}

	_, _ = fmt.Fprintf(p.Out, "Downloading %d %s (%s total) to %s\n\n",
		len(flatList), english.PluralWord(len(flatList), "file", "files"),
		humanize.IBytes(uint64(totalSize)), opts.output)
	_, _ = fmt.Fprintf(p.Out, "%-*s  %10s\n", nameWidth, "NAME", "SIZE")

	downloaded := 0
	for _, artifact := range flatList {
		rel, err := filepath.Rel(absOutput, filepath.Join(absOutput, artifact.Name))
		if err != nil || !filepath.IsLocal(rel) {
			_, _ = fmt.Fprintf(p.Out, "%-*s  %10s  %s path escapes output directory\n", nameWidth, artifact.Name, "", output.Red("   "+output.Sym().Cross))
			continue
		}
		outputPath := filepath.Join(absOutput, rel)
		size := humanize.IBytes(uint64(artifact.Size))

		if err := downloadArtifact(ctx, client, runID, artifact, outputPath, nameWidth, p.Quiet, p.Out); err != nil {
			_, _ = fmt.Fprintf(p.Out, "%-*s  %10s  %s %v\n", nameWidth, artifact.Name, size, output.Red("   "+output.Sym().Cross), err)
			continue
		}
		_, _ = fmt.Fprintf(p.Out, "%-*s  %10s  %s\n", nameWidth, artifact.Name, size, output.Green("   "+output.Sym().Check))
		downloaded++
	}

	_, _ = fmt.Fprintf(p.Out, "\n%s %s downloaded\n", output.Green(output.Sym().Check), english.Plural(downloaded, "artifact", ""))
	return nil
}

func downloadArtifact(ctx context.Context, client api.ClientInterface, runID string, artifact api.Artifact, outputPath string, nameWidth int, quiet bool, out io.Writer) error {
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
	if output.IsTerminal() && !quiet && artifact.Size > 0 {
		pw := output.NewProgressWriter(f, out, artifact.Name, humanize.IBytes(uint64(artifact.Size)), artifact.Size, nameWidth)
		w = pw
		defer pw.Clear()
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
