package auth

import (
	"errors"
	"fmt"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/spf13/cobra"
)

func newAuthLogoutCmd(f *cmdutil.Factory) *cobra.Command {
	var serverFlag string

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out from a TeamCity server",
		Example: `  teamcity auth logout
  teamcity auth logout --server https://old-server.example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogout(f, serverFlag)
		},
	}

	cmd.Flags().StringVarP(&serverFlag, "server", "s", "", "Server URL to log out from")

	_ = cmd.RegisterFlagCompletionFunc("server", completion.ConfiguredServers())

	return cmd
}

func runAuthLogout(f *cmdutil.Factory, serverFlag string) error {
	p := f.Printer

	var serverURL string
	if serverFlag != "" {
		serverURL = config.NormalizeURL(serverFlag)
		if _, ok := config.Get().Servers[serverURL]; !ok {
			return fmt.Errorf("server %q not found in configuration", serverURL)
		}
	} else {
		serverURL = config.GetServerURL()
		if serverURL == "" {
			return errors.New("not logged in to any server")
		}
	}

	if err := config.RemoveServer(serverURL); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(p.Out, "Logged out from %s\n", serverURL)
	return nil
}
