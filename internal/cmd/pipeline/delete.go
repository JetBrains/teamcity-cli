package pipeline

import (
	"errors"
	"fmt"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type deleteOptions struct {
	yes bool
}

func newPipelineDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete <pipeline-id>",
		Short: "Delete a pipeline",
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity pipeline delete CLI_MyPipeline
  teamcity pipeline delete CLI_MyPipeline --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineDelete(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&opts.yes, "force", false, "")
	cmdutil.DeprecateFlag(cmd, "force", "yes", "v1.0.0")

	return cmd
}

func runPipelineDelete(f *cmdutil.Factory, id string, opts *deleteOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	pipeline, err := client.GetPipeline(id)
	if err != nil {
		return err
	}

	if !opts.yes {
		if !f.IsInteractive() {
			return errors.New("--yes is required in non-interactive mode")
		}
		var confirm bool
		if err := cmdutil.Confirm(fmt.Sprintf("Delete pipeline %q (%s)?", pipeline.Name, pipeline.ID), &confirm); err != nil {
			return err
		}
		if !confirm {
			return nil
		}
	}

	if err := client.DeletePipeline(id); err != nil {
		return fmt.Errorf("failed to delete pipeline %s: %w", id, err)
	}

	f.Printer.Success("Deleted pipeline %q (%s)", pipeline.Name, pipeline.ID)
	return nil
}
