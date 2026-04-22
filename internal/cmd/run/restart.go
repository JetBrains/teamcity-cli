package run

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type runRestartOptions struct {
	watchFlags
	web bool
}

func newRunRestartCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runRestartOptions{}

	cmd := &cobra.Command{
		Use:   "restart <id>",
		Short: "Restart a run",
		Long: `Re-queue a run with the same job, branch, revision, and parameters.

The new run is a fresh build (new ID, new number) that reuses the
source configuration from the original. Use --watch to stream the
restarted run until it completes.`,
		Args: cobra.ExactArgs(1),
		Example: `  teamcity run restart 12345
  teamcity run restart 12345 --watch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunRestart(f, args[0], opts)
		},
	}

	opts.addToCmd(cmd)
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")

	return cmd
}

func runRunRestart(f *cmdutil.Factory, runID string, opts *runRestartOptions) error {
	p := f.Printer
	opts.resolve()

	client, err := f.Client()
	if err != nil {
		return err
	}

	originalBuild, err := client.GetBuild(f.Context(), runID)
	if err != nil {
		return fmt.Errorf("failed to get run: %w", err)
	}

	newBuild, err := client.RunBuild(originalBuild.BuildTypeID, api.RunBuildOptions{
		Branch: originalBuild.BranchName,
	})
	if err != nil {
		return fmt.Errorf("failed to trigger run: %w", err)
	}

	printQueuedRun(p, newBuild, fmt.Sprintf("%s (restart of %d)", originalBuild.BuildTypeID, originalBuild.ID))
	_, _ = fmt.Fprintf(p.Out, "  Job: %s\n", originalBuild.BuildTypeID)
	if originalBuild.BranchName != "" {
		_, _ = fmt.Fprintf(p.Out, "  Branch: %s\n", originalBuild.BranchName)
	}
	p.Info("  URL: %s", newBuild.WebURL)

	return afterQueue(f, newBuild, opts.web, &opts.watchFlags)
}
