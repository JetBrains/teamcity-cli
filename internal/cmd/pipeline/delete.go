package pipeline

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type deleteOptions struct {
	force bool
}

func newPipelineDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete <pipeline-id>",
		Short: "Delete a pipeline",
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity pipeline delete CLI_MyPipeline
  teamcity pipeline delete CLI_MyPipeline --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineDelete(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.force, "force", false, "Skip confirmation prompt")

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

	if !opts.force {
		if !f.IsInteractive() {
			return fmt.Errorf("--force is required in non-interactive mode")
		}
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Delete pipeline %q (%s)?", pipeline.Name, pipeline.ID),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
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
