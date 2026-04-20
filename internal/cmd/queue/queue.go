package queue

import (
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage build queue",
		Long: `List and manage the TeamCity build queue.

The queue holds runs that are waiting for a compatible agent. Use
these commands to inspect pending runs, reorder them, approve guarded
runs, or remove entries.

See: https://www.jetbrains.com/help/teamcity/build-queue.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newQueueListCmd(f))
	cmd.AddCommand(newQueueRemoveCmd(f))
	cmd.AddCommand(newQueueTopCmd(f))
	cmd.AddCommand(newQueueApproveCmd(f))

	return cmd
}
