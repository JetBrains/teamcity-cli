package queue

import (
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage build queue",
		Long:  `List and manage the TeamCity build queue.`,
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newQueueListCmd(f))
	cmd.AddCommand(newQueueRemoveCmd(f))
	cmd.AddCommand(newQueueTopCmd(f))
	cmd.AddCommand(newQueueApproveCmd(f))

	return cmd
}
