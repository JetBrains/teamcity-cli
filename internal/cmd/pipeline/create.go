package pipeline

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type createOptions struct {
	project string
	vcsRoot string
	file    string
}

func newPipelineCreateCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new pipeline from YAML",
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity pipeline create my-pipeline --project CLI --vcs-root MyVcsRoot
  teamcity pipeline create my-pipeline --project CLI --file pipeline.yml
  teamcity pipeline create my-pipeline --project CLI  # interactive VCS root selection`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineCreate(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Parent project ID (required)")
	cmd.Flags().StringVar(&opts.vcsRoot, "vcs-root", "", "VCS root ID (interactive selection if omitted)")
	cmd.Flags().StringVarP(&opts.file, "file", "f", ".teamcity.yml", "Path to pipeline YAML file")
	_ = cmd.MarkFlagRequired("project")

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
	if vcsRootID == "" {
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
	f.Printer.Success("Created pipeline %q (%s)", pipeline.Name, pipeline.ID)
	if pipeline.WebURL != "" {
		_, _ = fmt.Fprintf(f.Printer.Out, "  %s\n", pipeline.WebURL)
	}

	return nil
}

func selectVcsRoot(f *cmdutil.Factory, client api.ClientInterface, projectID string) (string, error) {
	if !f.IsInteractive() {
		return "", fmt.Errorf("--vcs-root is required (or use interactive mode)")
	}

	roots, err := client.GetVcsRoots(api.VcsRootsOptions{Project: projectID, Limit: 100})
	if err != nil {
		return "", fmt.Errorf("failed to list VCS roots: %w", err)
	}

	if len(roots.VcsRoot) == 0 {
		return "", fmt.Errorf("no VCS roots found in project %s, create one first", projectID)
	}

	options := make([]string, len(roots.VcsRoot))
	ids := make([]string, len(roots.VcsRoot))
	for i, r := range roots.VcsRoot {
		options[i] = fmt.Sprintf("%s (%s)", r.Name, r.ID)
		ids[i] = r.ID
	}

	var selected int
	prompt := &survey.Select{
		Message: "Select VCS root:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return "", err
	}

	return ids[selected], nil
}
