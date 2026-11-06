package pipeline

import (
	"fmt"
	"os"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/onboarding"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

type createOptions struct {
	project string
	vcsRoot string
	file    string
	json    bool
}

func newPipelineCreateCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new pipeline from YAML",
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity pipeline create my-pipeline --project CLI --vcs-root MyVcsRoot
  teamcity pipeline create my-pipeline --project CLI --file pipeline.yml
  teamcity pipeline create my-pipeline --project CLI  # interactive VCS root selection
  teamcity pipeline create my-pipeline --project CLI --no-input  # no VCS root, runs with a patch of the local repository`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineCreate(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Parent project ID (required)")
	cmd.Flags().StringVar(&opts.vcsRoot, "vcs-root", "", "VCS root ID (interactive selection if omitted; none with --no-input or --json)")
	cmd.Flags().StringVarP(&opts.file, "file", "f", ".teamcity.yml", "Path to pipeline YAML file")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagFilename("file", "yml", "yaml")

	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func runPipelineCreate(f *cmdutil.Factory, name string, opts *createOptions) error {
	data, err := os.ReadFile(opts.file)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", opts.file, err)
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	vcsRootID := opts.vcsRoot
	// --json is for scripts: no picker, so stdout stays valid JSON
	if vcsRootID == "" && !opts.json {
		selected, err := selectVcsRoot(f, client, opts.project)
		if err != nil {
			return err
		}
		vcsRootID = selected
	}

	pipeline, err := client.CreatePipeline(opts.project, name, string(data), vcsRootID)
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}

	f.Analytics.Track(analytics.GroupPipeline, analytics.EventCreated, map[string]any{"is_from_file": true})
	onboarding.NotifyPipelineCreated(f, client, pipeline.ID)
	if opts.json {
		return f.Printer.PrintJSON(pipeline)
	}
	f.Printer.Success("Created pipeline %q (%s)", pipeline.Name, pipeline.ID)
	if pipeline.WebURL != "" {
		_, _ = fmt.Fprintf(f.Printer.Out, "  %s\n", pipeline.WebURL)
	}

	return nil
}

// selectVcsRoot returns the VCS root to attach, or "" for a pipeline without a VCS root.
func selectVcsRoot(f *cmdutil.Factory, client api.ClientInterface, projectID string) (string, error) {
	if !f.IsInteractive() {
		return "", nil
	}

	roots, _, err := client.GetVcsRoots(api.VcsRootsOptions{Project: projectID, Limit: 100})
	if err != nil {
		return "", fmt.Errorf("failed to list VCS roots: %w", err)
	}

	options := make([]huh.Option[string], 0, len(roots.VcsRoot)+1)
	for _, r := range roots.VcsRoot {
		options = append(options, huh.NewOption(fmt.Sprintf("%s (%s)", r.Name, r.ID), r.ID))
	}
	options = append(options, huh.NewOption("No VCS root (run with a patch of the local repository)", ""))

	var selected string
	if err := cmdutil.Select(f.Printer, "Select VCS root", options, &selected); err != nil {
		return "", err
	}

	return selected, nil
}
