package pipeline

import (
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage pipelines (YAML configurations)",
		Long: `List, view, validate, and manage TeamCity pipelines.

Pipelines are YAML-defined workflows that orchestrate one or more jobs
end-to-end. Use these commands to validate pipeline YAML against the
server schema, push changes, and pull the current definition.

See: https://www.jetbrains.com/help/teamcity/create-and-edit-pipelines.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newPipelineListCmd(f))
	cmd.AddCommand(newPipelineViewCmd(f))
	cmd.AddCommand(newPipelineValidateCmd(f))
	cmd.AddCommand(newPipelineCreateCmd(f))
	cmd.AddCommand(newPipelineDeleteCmd(f))
	cmd.AddCommand(newPipelinePullCmd(f))
	cmd.AddCommand(newPipelinePushCmd(f))
	cmd.AddCommand(newPipelineSchemaCmd(f))

	return cmd
}
