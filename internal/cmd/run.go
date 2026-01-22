package cmd

import (
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Manage runs (builds)",
		Long:  `List, view, start, and manage TeamCity runs (builds).`,
	}

	cmd.AddCommand(newRunListCmd())
	cmd.AddCommand(newRunViewCmd())
	cmd.AddCommand(newRunStartCmd())
	cmd.AddCommand(newRunCancelCmd())
	cmd.AddCommand(newRunWatchCmd())
	cmd.AddCommand(newRunRestartCmd())
	cmd.AddCommand(newRunDownloadCmd())
	cmd.AddCommand(newRunLogCmd())
	cmd.AddCommand(newRunPinCmd())
	cmd.AddCommand(newRunUnpinCmd())
	cmd.AddCommand(newRunTagCmd())
	cmd.AddCommand(newRunUntagCmd())
	cmd.AddCommand(newRunCommentCmd())
	cmd.AddCommand(newRunChangesCmd())
	cmd.AddCommand(newRunTestsCmd())

	return cmd
}
