package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
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
	var insecureStorage bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a TeamCity server",
		Long: `Authenticate with a TeamCity server using an access token.

This will:
1. Prompt for your TeamCity server URL
2. Open your browser to generate an access token
3. Validate and store the token securely

The token is stored in your system keyring (macOS Keychain, GNOME Keyring,
Windows Credential Manager) when available. Use --insecure-storage to store
the token in plain text in the config file instead.

For CI/CD, use environment variables instead:
  export TEAMCITY_URL="https://teamcity.example.com"
  export TEAMCITY_TOKEN="your-access-token"

When running inside a TeamCity build, authentication is automatic using
build-level credentials from the build properties file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogin(serverURL, token, insecureStorage)
		},
	}

	cmd.Flags().StringVarP(&serverURL, "server", "s", "", "TeamCity server URL")
	cmd.Flags().StringVarP(&token, "token", "t", "", "Access token")
	cmd.Flags().BoolVar(&insecureStorage, "insecure-storage", false, "Store token in plain text config file instead of system keyring")

	return cmd
}

func runAuthLogin(serverURL, token string, insecureStorage bool) error {
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

	warnInsecureHTTP(serverURL, "authentication token")
	output.Infof("Validating... ")

	client := api.NewClient(serverURL, token, api.WithDebugFunc(output.Debug))
	user, err := client.GetCurrentUser()
	if err != nil {
		output.Info("%s", output.Red("✗"))
		return tcerrors.AuthenticationFailed()
	}

	output.Info("%s", output.Green("✓"))

	insecureFallback, err := config.SetServerWithKeyring(serverURL, token, user.Username, insecureStorage)
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	output.Success("Logged in as %s", output.Cyan(user.Name))
	if insecureFallback {
		fmt.Printf("%s Token stored in plain text at %s\n", output.Yellow("!"), config.ConfigPath())
	} else {
		fmt.Printf("%s Token stored in system keyring\n", output.Green("✓"))
	}

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
	token, tokenSource := config.GetTokenWithSource()

	if serverURL != "" && token != "" {
		return showExplicitAuthStatus(serverURL, token, tokenSource)
	}

	if buildAuth, ok := config.GetBuildAuth(); ok {
		return showBuildAuthStatus(buildAuth)
	}

	fmt.Println(output.Red("✗"), "Not logged in to any TeamCity server")
	fmt.Println("\nRun", output.Cyan("teamcity auth login"), "to authenticate")
	if config.IsBuildEnvironment() {
		fmt.Println("\n" + output.Yellow("!") + " Build environment detected but credentials not found in properties file")
	}
	return nil
}

func tokenSourceLabel(source string) string {
	switch source {
	case "env":
		return "environment variable"
	case "keyring":
		return "system keyring"
	case "config":
		return config.ConfigPath()
	default:
		return "unknown"
	}
}

func showExplicitAuthStatus(serverURL, token, tokenSource string) error {
	warnInsecureHTTP(serverURL, "authentication token")
	client := api.NewClient(serverURL, token, api.WithDebugFunc(output.Debug))
	user, err := client.GetCurrentUser()
	if err != nil {
		fmt.Printf("%s Server: %s\n", output.Red("✗"), serverURL)
		fmt.Println("  Token is invalid or expired")
		return nil
	}

	fmt.Printf("%s Logged in to %s\n", output.Green("✓"), output.Cyan(serverURL))
	fmt.Printf("  User: %s (%s) · %s\n", user.Name, user.Username, tokenSourceLabel(tokenSource))

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

func showBuildAuthStatus(buildAuth *config.BuildAuth) error {
	warnInsecureHTTP(buildAuth.ServerURL, "credentials")
	client := api.NewClientWithBasicAuth(buildAuth.ServerURL, buildAuth.Username, buildAuth.Password, api.WithDebugFunc(output.Debug))
	server, err := client.GetServer()
	if err != nil {
		fmt.Printf("%s Server: %s\n", output.Red("✗"), buildAuth.ServerURL)
		fmt.Println("  Build credentials are invalid")
		return nil
	}

	fmt.Printf("%s Connected to %s\n", output.Green("✓"), output.Cyan(buildAuth.ServerURL))
	fmt.Printf("  Auth: %s\n", output.Faint("Build-level credentials"))
	fmt.Printf("  Scope: %s\n", output.Faint("Build-level access"))
	fmt.Printf("  Server: TeamCity %d.%d (build %s)\n", server.VersionMajor, server.VersionMinor, server.BuildNumber)

	return nil
}
