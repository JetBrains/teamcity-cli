package pipeline

import (
	"fmt"
	"os"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/spf13/cobra"
)

func newPipelinePushCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push <pipeline-id> [file]",
		Short: "Upload pipeline YAML",
		Args:  cobra.RangeArgs(1, 2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return completion.LinkedJobs()(cmd, args, toComplete)
			}
			return []string{"yml", "yaml"}, cobra.ShellCompDirectiveFilterFileExt
		},
		Example: `  teamcity pipeline push CLI_CiCd
  teamcity pipeline push CLI_CiCd .teamcity.yml
  teamcity pipeline push CLI_CiCd pipeline.yml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file := ".teamcity.yml"
			if len(args) > 1 {
				file = args[1]
			}
			return runPipelinePush(f, args[0], file)
		},
	}

	return cmd
}

func runPipelinePush(f *cmdutil.Factory, id, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file, err)
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	yaml, err := client.GetPipelineYAML(id)
	if err != nil {
		return fmt.Errorf("failed to check pipeline: %w", err)
	}
	if yaml == "" {
		return api.Validation(
			fmt.Sprintf("pipeline %s stores its YAML in the VCS repository", id),
			"Commit .teamcity.yml to your repo directly",
		)
	}

	if err := client.UpdatePipelineYAML(id, string(data)); err != nil {
		return fmt.Errorf("failed to update pipeline %s: %w", id, err)
	}

	f.Analytics.Track(analytics.GroupPipeline, analytics.EventSynced, map[string]any{"action": analytics.PipelineActionPush})
	f.Printer.Success("Updated pipeline %s", id)
	return nil
}
