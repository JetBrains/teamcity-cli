package run

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type runCancelOptions struct {
	comment string
	yes     bool
}

func newRunCancelCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runCancelOptions{}

	cmd := &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a run",
		Long: `Cancel a running or queued run.

Prompts for confirmation when run interactively without --yes or
--comment. The cancellation comment is stored on the run and shown
in the TeamCity UI.`,
		Args: cobra.ExactArgs(1),
		Example: `  teamcity run cancel 12345
  teamcity run cancel 12345 --comment "Canceling for hotfix"
  teamcity run cancel 12345 --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunCancel(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.comment, "comment", "m", "", "Comment for cancellation")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVarP(&opts.yes, "force", "f", false, "")
	cmdutil.DeprecateFlag(cmd, "force", "yes", "v1.0.0")

	return cmd
}

func runRunCancel(f *cmdutil.Factory, runID string, opts *runCancelOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	needsConfirmation := !opts.yes && opts.comment == "" && f.IsInteractive()

	if needsConfirmation {
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Cancel run #%s?", runID),
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

	comment := opts.comment
	if comment == "" {
		comment = "Canceled via teamcity CLI"
	}

	if err := client.CancelBuild(runID, comment); err != nil {
		return err
	}

	f.Printer.Success("Canceled #%s", runID)
	return nil
}
