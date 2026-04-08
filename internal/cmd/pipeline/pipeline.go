package pipeline

import (
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage pipelines (YAML configurations)",
		Long:  `List, view, validate, and manage TeamCity pipelines.`,
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newPipelineListCmd(f))
	cmd.AddCommand(newPipelineViewCmd(f))
	cmd.AddCommand(newPipelineValidateCmd(f))
	cmd.AddCommand(newPipelineCreateCmd(f))
	cmd.AddCommand(newPipelineDeleteCmd(f))
	cmd.AddCommand(newPipelinePullCmd(f))
	cmd.AddCommand(newPipelinePushCmd(f))

	return cmd
}
