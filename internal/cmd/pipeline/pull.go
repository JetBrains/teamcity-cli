package pipeline

import (
	"fmt"
	"os"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type pullOptions struct {
	output string
}

func newPipelinePullCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &pullOptions{}

	cmd := &cobra.Command{
		Use:   "pull <pipeline-id>",
		Short: "Download pipeline YAML",
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity pipeline pull CLI_CiCd
  teamcity pipeline pull CLI_CiCd -o .teamcity.yml
  teamcity pipeline pull CLI_CiCd > pipeline.yml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelinePull(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Write YAML to file instead of stdout")

	return cmd
}

func runPipelinePull(f *cmdutil.Factory, id string, opts *pullOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	yaml, err := client.GetPipelineYAML(id)
	if err != nil {
		return fmt.Errorf("failed to get pipeline YAML: %w", err)
	}

	if yaml == "" {
		return api.Validation(
			fmt.Sprintf("pipeline %s stores its YAML in the VCS repository", id),
			"Edit .teamcity.yml in your repo directly",
		)
	}

	f.Analytics.Track(analytics.GroupPipeline, analytics.EventSynced, map[string]any{"action": analytics.PipelineActionPull})

	if opts.output != "" {
		if err := os.WriteFile(opts.output, []byte(yaml), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", opts.output, err)
		}
		f.Printer.Success("Written to %s", opts.output)
		return nil
	}

	_, _ = fmt.Fprint(f.Printer.Out, yaml)
	return nil
}
