// Package server implements commands for managing the TeamCity server.
package server

import (
	"github.com/JetBrains/teamcity-cli/internal/cmd/plugin"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmd creates the server command.
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage the TeamCity server",
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(plugin.NewCmd(f))
	return cmd
}
