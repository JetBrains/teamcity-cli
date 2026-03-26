package auth

import (
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with TeamCity",
		Long:  `Manage authentication state for TeamCity servers.`,
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newAuthLoginCmd(f))
	cmd.AddCommand(newAuthLogoutCmd(f))
	cmd.AddCommand(newAuthStatusCmd(f))

	return cmd
}
