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
		Long:  `Download artifacts from a completed run.`,
		Args:  cobra.ExactArgs(1),
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

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
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
			_, _ = fmt.Fprintf(p.Out, "%-*s  %10s  %s path escapes output directory\n", nameWidth, artifact.Name, "", output.Red("   ✗"))
			continue
		}
		outputPath := filepath.Join(absOutput, rel)
		size := humanize.IBytes(uint64(artifact.Size))

		if err := downloadArtifact(ctx, client, runID, artifact, outputPath, nameWidth, p.Quiet, p.Out); err != nil {
			_, _ = fmt.Fprintf(p.Out, "%-*s  %10s  %s %v\n", nameWidth, artifact.Name, size, output.Red("   ✗"), err)
			continue
		}
		_, _ = fmt.Fprintf(p.Out, "%-*s  %10s  %s\n", nameWidth, artifact.Name, size, output.Green("   ✓"))
		downloaded++
	}

	_, _ = fmt.Fprintf(p.Out, "\n%s %s downloaded\n", output.Green("✓"), english.Plural(downloaded, "artifact", ""))
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
		pw := &progressWriter{
			w:         f,
			out:       out,
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
	out        io.Writer
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
		_, _ = fmt.Fprintf(p.out, "\r%-*s  %10s  %3d%%", p.nameWidth, p.name, p.size, pct)
	}
	return n, err
}

func (p *progressWriter) clear() {
	_, _ = fmt.Fprint(p.out, "\r\033[K")
}
