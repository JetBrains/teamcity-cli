package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/api"
	"github.com/tiulpin/teamcity-cli/internal/config"
	tcerrors "github.com/tiulpin/teamcity-cli/internal/errors"
	"github.com/tiulpin/teamcity-cli/internal/output"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with TeamCity",
		Long:  `Manage authentication state for TeamCity servers.`,
		Args:  cobra.NoArgs,
		RunE:  subcommandRequired,
	}

	cmd.AddCommand(newAuthLoginCmd())
	cmd.AddCommand(newAuthLogoutCmd())
	cmd.AddCommand(newAuthStatusCmd())

	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var serverURL string
	var token string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a TeamCity server",
		Long: `Authenticate with a TeamCity server using an access token.

You can generate an access token in your TeamCity profile settings.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogin(serverURL, token)
		},
	}

	cmd.Flags().StringVarP(&serverURL, "server", "s", "", "TeamCity server URL")
	cmd.Flags().StringVarP(&token, "token", "t", "", "Access token")

	return cmd
}

func runAuthLogin(serverURL, token string) error {
	isInteractive := !NoInput && output.IsStdinTerminal()

	if serverURL == "" {
		if !isInteractive {
			return tcerrors.RequiredFlag("server")
		}
		prompt := &survey.Input{
			Message: "TeamCity server URL:",
			Help:    "e.g., https://teamcity.example.com",
		}
		if err := survey.AskOne(prompt, &serverURL, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	serverURL = strings.TrimSuffix(serverURL, "/")
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		serverURL = "https://" + serverURL
	}

	if token == "" {
		if !isInteractive {
			return tcerrors.RequiredFlag("token")
		}

		tokenURL := fmt.Sprintf("%s/profile.html?item=accessTokens", serverURL)

		fmt.Println()
		fmt.Println(output.Yellow("!"), "To authenticate, you need an access token.")
		fmt.Printf("  Generate one at: %s\n", tokenURL)
		fmt.Println()

		openBrowser := false
		confirmPrompt := &survey.Confirm{
			Message: "Open browser to generate token?",
			Default: true,
		}
		if err := survey.AskOne(confirmPrompt, &openBrowser); err != nil {
			return err
		}

		if openBrowser {
			if err := browser.OpenURL(tokenURL); err != nil {
				fmt.Printf("  Could not open browser. Please visit: %s\n", tokenURL)
			} else {
				fmt.Println(output.Green("  ✓"), "Opened browser")
			}
			fmt.Println()
		}

		tokenPrompt := &survey.Password{
			Message: "Paste your access token:",
		}
		if err := survey.AskOne(tokenPrompt, &token, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	output.Infof("Validating... ")

	client := api.NewClient(serverURL, token)
	user, err := client.GetCurrentUser()
	if err != nil {
		output.Info("%s", output.Red("✗"))
		return tcerrors.AuthenticationFailed()
	}

	output.Info("%s", output.Green("✓"))

	if err := config.SetServer(serverURL, token, user.Username); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	output.Success("Logged in as %s", output.Cyan(user.Name))
	output.Info("\nConfiguration saved to %s", config.ConfigPath())

	return nil
}

func newAuthLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out from a TeamCity server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogout()
		},
	}

	return cmd
}

func runAuthLogout() error {
	serverURL := config.GetServerURL()
	if serverURL == "" {
		return fmt.Errorf("not logged in to any server")
	}

	if err := config.RemoveServer(serverURL); err != nil {
		return err
	}

	fmt.Printf("Logged out from %s\n", serverURL)
	return nil
}

func newAuthStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus()
		},
	}

	return cmd
}

func runAuthStatus() error {
	serverURL := config.GetServerURL()
	if serverURL == "" {
		fmt.Println(output.Red("✗"), "Not logged in to any TeamCity server")
		fmt.Println("\nRun", output.Cyan("tc auth login"), "to authenticate")
		return nil
	}

	token := config.GetToken()
	if token == "" {
		fmt.Printf("%s Server: %s\n", output.Red("✗"), serverURL)
		fmt.Println("  No token configured")
		return nil
	}

	client := api.NewClient(serverURL, token)
	user, err := client.GetCurrentUser()
	if err != nil {
		fmt.Printf("%s Server: %s\n", output.Red("✗"), serverURL)
		fmt.Println("  Token is invalid or expired")
		return nil
	}

	fmt.Printf("%s Logged in to %s\n", output.Green("✓"), output.Cyan(serverURL))
	fmt.Printf("  User: %s (%s)\n", user.Name, user.Username)

	server, err := client.ServerVersion()
	if err == nil {
		fmt.Printf("  Server: TeamCity %d.%d (build %s)\n", server.VersionMajor, server.VersionMinor, server.BuildNumber)

		if err := client.CheckVersion(); err != nil {
			fmt.Printf("  %s %s\n", output.Yellow("!"), err.Error())
		} else {
			fmt.Printf("  %s API compatible\n", output.Green("✓"))
		}
	}

	return nil
}
