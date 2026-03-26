package pool

import (
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage agent pools",
		Long:  `List agent pools and manage project assignments.`,
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newPoolListCmd(f))
	cmd.AddCommand(newPoolViewCmd(f))
	cmd.AddCommand(newPoolLinkCmd(f))
	cmd.AddCommand(newPoolUnlinkCmd(f))

	return cmd
}
