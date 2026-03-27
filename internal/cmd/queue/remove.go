package queue

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type queueRemoveOptions struct {
	force bool
}

func newQueueRemoveCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &queueRemoveOptions{}

	cmd := &cobra.Command{
		Use:   "remove <run-id>",
		Short: "Remove a run from the queue",
		Long:  `Remove a queued run from the TeamCity queue.`,
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity queue remove 12345
  teamcity queue remove 12345 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueueRemove(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runQueueRemove(f *cmdutil.Factory, runID string, opts *queueRemoveOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	needsConfirmation := !opts.force && f.IsInteractive()

	if needsConfirmation {
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Remove run %s from queue?", runID),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			f.Printer.Info("Canceled")
			return nil
		}
	}

	if err := client.RemoveFromQueue(runID); err != nil {
		return fmt.Errorf("failed to remove run from queue: %w", err)
	}

	f.Printer.Success("Removed run %s from queue", runID)
	return nil
}
