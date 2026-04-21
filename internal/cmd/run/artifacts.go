package run

import (
	"context"
	"fmt"
	"path"
	"strconv"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/spf13/cobra"
)

type runArtifactsOptions struct {
	job  string
	path string
	json bool
}

func newRunArtifactsCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runArtifactsOptions{}

	cmd := &cobra.Command{
		Use:   "artifacts [id]",
		Short: "List artifacts",
		Long: `List artifacts from a run without downloading them.

Shows artifact names and sizes. Use teamcity run download to download artifacts.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && cmd.Flags().Changed("job") {
				return api.MutuallyExclusive("id", "job")
			}
			return cobra.MaximumNArgs(1)(cmd, args)
		},
		Example: `  teamcity run artifacts 12345
  teamcity run artifacts 12345 --json
  teamcity run artifacts 12345 --path html_reports/coverage
  teamcity run artifacts --job MyBuild`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var runID string
			if len(args) > 0 {
				runID = args[0]
			}
			return runRunArtifacts(f, runID, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Use this job's latest")
	cmd.Flags().StringVarP(&opts.path, "path", "p", "", "Browse artifacts under this subdirectory")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runRunArtifacts(f *cmdutil.Factory, runID string, opts *runArtifactsOptions) error {
	p := f.Printer
	client, err := f.Client()
	if err != nil {
		return err
	}

	if opts.job != "" {
		runs, err := client.GetBuilds(context.Background(), api.BuildsOptions{
			BuildTypeID: opts.job,
			State:       "finished",
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
		p.Info("Listing artifacts for run %s  #%s", runID, runs.Builds[0].Number)
	} else if runID == "" {
		return fmt.Errorf("run ID required (or use --job to get latest run)")
	}

	artifacts, err := client.GetArtifacts(context.Background(), runID, opts.path)
	if err != nil {
		return fmt.Errorf("failed to get artifacts: %w", err)
	}

	if opts.json {
		return p.PrintJSON(artifacts)
	}

	if artifacts.Count == 0 {
		p.Empty("No artifacts found for this run", output.TipNoArtifactsFor(runID))
		return nil
	}

	flatList, totalSize := flattenArtifacts(artifacts.File, "")

	displayNames := make([]string, len(flatList))
	for i, a := range flatList {
		displayNames[i] = a.Name
		if a.Content == nil {
			displayNames[i] += "/"
		}
	}

	nameWidth := 4 // "NAME"
	for _, name := range displayNames {
		nameWidth = max(nameWidth, len(name))
	}

	_, _ = fmt.Fprintf(p.Out, "ARTIFACTS (%d %s, %s total)\n\n", len(flatList), english.PluralWord(len(flatList), "file", "files"), humanize.IBytes(uint64(totalSize)))
	_, _ = fmt.Fprintf(p.Out, "%-*s  %10s\n", nameWidth, "NAME", "SIZE")

	for i, a := range flatList {
		size := ""
		if a.Size > 0 {
			size = humanize.IBytes(uint64(a.Size))
		}
		_, _ = fmt.Fprintf(p.Out, "%-*s  %s\n", nameWidth, displayNames[i], output.Faint(fmt.Sprintf("%10s", size)))
	}

	if opts.path != "" {
		_, _ = fmt.Fprintf(p.Out, "\nDownload dir: teamcity run download %s --path %s\n", runID, opts.path)
	} else {
		_, _ = fmt.Fprintf(p.Out, "\nDownload all: teamcity run download %s\n", runID)
	}
	_, _ = fmt.Fprintf(p.Out, "Download one: teamcity run download %s -a \"<name>\"\n", runID)
	return nil
}

// Shared artifact helpers used by both artifacts and download commands.

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
			result = append(result, api.Artifact{Name: name, Size: a.Size, Content: a.Content, Children: a.Children})
			totalSize += a.Size
		}
	}
	return result, totalSize
}

const maxArtifactDepth = 20

func fetchAllArtifacts(ctx context.Context, client api.ClientInterface, runID, basePath string) ([]api.Artifact, int64, error) {
	return fetchArtifactsRecursive(ctx, client, runID, basePath, 0)
}

func fetchArtifactsRecursive(ctx context.Context, client api.ClientInterface, runID, basePath string, depth int) ([]api.Artifact, int64, error) {
	if depth > maxArtifactDepth {
		return nil, 0, fmt.Errorf("artifact tree exceeds maximum depth (%d)", maxArtifactDepth)
	}

	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}

	artifacts, err := client.GetArtifacts(ctx, runID, basePath)
	if err != nil {
		return nil, 0, err
	}

	var result []api.Artifact
	var totalSize int64
	for _, a := range artifacts.File {
		name := a.Name
		if basePath != "" {
			name = basePath + "/" + a.Name
		}
		if a.Content != nil {
			result = append(result, api.Artifact{Name: name, Size: a.Size, Content: a.Content})
			totalSize += a.Size
		} else {
			nested, size, err := fetchArtifactsRecursive(ctx, client, runID, name, depth+1)
			if err != nil {
				return nil, 0, err
			}
			result = append(result, nested...)
			totalSize += size
		}
	}
	return result, totalSize, nil
}

func filterArtifacts(artifacts []api.Artifact, pattern string) ([]api.Artifact, int64, error) {
	if _, err := path.Match(pattern, ""); err != nil {
		return nil, 0, fmt.Errorf("invalid artifact pattern %q: %w", pattern, err)
	}

	var filtered []api.Artifact
	var filteredSize int64

	for _, a := range artifacts {
		if matched, _ := path.Match(pattern, a.Name); matched {
			filtered = append(filtered, a)
			filteredSize += a.Size
			continue
		}
		if matched, _ := path.Match(pattern, path.Base(a.Name)); matched {
			filtered = append(filtered, a)
			filteredSize += a.Size
		}
	}

	return filtered, filteredSize, nil
}
