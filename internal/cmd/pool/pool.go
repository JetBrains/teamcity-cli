package pool

import (
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage agent pools",
		Long: `List agent pools and manage project assignments.

Agent pools group build agents and bind them to specific projects,
so a project's builds only run on approved agents. Use these commands
to inspect pools and link or unlink projects.

See: https://www.jetbrains.com/help/teamcity/configuring-agent-pools.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newPoolListCmd(f))
	cmd.AddCommand(newPoolViewCmd(f))
	cmd.AddCommand(newPoolLinkCmd(f))
	cmd.AddCommand(newPoolUnlinkCmd(f))

	return cmd
}
