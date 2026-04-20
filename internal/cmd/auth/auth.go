package auth

import (
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with TeamCity",
		Long: `Log in, log out, and inspect authentication state for TeamCity servers.

Credentials are stored in the system keyring by default and can be
overridden via TEAMCITY_URL and TEAMCITY_TOKEN environment variables
for CI/CD usage.

See: https://www.jetbrains.com/help/teamcity/configuring-your-personal-settings.html#Managing+Access+Tokens`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newAuthLoginCmd(f))
	cmd.AddCommand(newAuthLogoutCmd(f))
	cmd.AddCommand(newAuthStatusCmd(f))

	return cmd
}
