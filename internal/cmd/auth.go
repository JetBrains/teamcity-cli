package cmd

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"slices"

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
	var guest bool
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

For guest access (read-only, no token needed; must be enabled on the server):
  teamcity auth login -s https://teamcity.example.com --guest

For CI/CD, use environment variables instead:
  export TEAMCITY_URL="https://teamcity.example.com"
  export TEAMCITY_TOKEN="your-access-token"
  # Or for guest access:
  export TEAMCITY_URL="https://teamcity.example.com"
  export TEAMCITY_GUEST=1

When running inside a TeamCity build, authentication is automatic using
build-level credentials from the build properties file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if guest {
				return runAuthLoginGuest(serverURL, token)
			}
			return runAuthLogin(serverURL, token, insecureStorage)
		},
	}

	cmd.Flags().StringVarP(&serverURL, "server", "s", "", "TeamCity server URL")
	cmd.Flags().StringVarP(&token, "token", "t", "", "Access token")
	cmd.Flags().BoolVar(&guest, "guest", false, "Use guest authentication (no token needed, must be enabled on the server)")
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

	serverURL = config.NormalizeURL(serverURL)

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

func runAuthLoginGuest(serverURL, token string) error {
	if token != "" {
		return tcerrors.WithSuggestion(
			"cannot use --guest with --token",
			"Use either --guest for guest access or --token for token authentication",
		)
	}

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

	serverURL = config.NormalizeURL(serverURL)

	warnInsecureHTTP(serverURL, "guest access")
	output.Infof("Validating guest access... ")

	client := api.NewGuestClient(serverURL, api.WithDebugFunc(output.Debug))
	server, err := client.GetServer()
	if err != nil {
		output.Info("%s", output.Red("✗"))
		return tcerrors.WithSuggestion(
			"Guest access validation failed",
			"Verify the server URL and that guest access is enabled on the server",
		)
	}

	output.Info("%s", output.Green("✓"))

	if err := config.SetGuestServer(serverURL); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	output.Success("Guest access to %s", output.Cyan(serverURL))
	fmt.Printf("  Server: TeamCity %d.%d (build %s)\n", server.VersionMajor, server.VersionMinor, server.BuildNumber)

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
	if envURL := os.Getenv(config.EnvServerURL); envURL != "" {
		envURL = config.NormalizeURL(envURL)
		if config.IsGuestAuth() {
			showGuestAuthStatus(envURL, "")
			return nil
		}
		if envToken := os.Getenv(config.EnvToken); envToken != "" {
			showExplicitAuthStatus(envURL, envToken, "env", "")
			return nil
		}
	}

	if buildAuth, ok := config.GetBuildAuth(); ok {
		showBuildAuthStatus(buildAuth)
		return nil
	}

	cfg := config.Get()
	shown := 0

	urls := sortedServerURLs(cfg)
	for i, serverURL := range urls {
		if i > 0 {
			fmt.Println()
		}
		sc := cfg.Servers[serverURL]
		suffix := ""
		if len(urls) > 1 && serverURL == cfg.DefaultServer {
			suffix = " (default)"
		}

		if sc.Guest {
			showGuestAuthStatus(serverURL, suffix)
		} else if token, src := config.GetTokenForServer(serverURL); token != "" {
			showExplicitAuthStatus(serverURL, token, src, suffix)
		} else {
			fmt.Printf("%s %s%s\n", output.Red("✗"), serverURL, suffix)
			fmt.Println("  Token is missing or could not be retrieved")
			printLoginHint(serverURL)
		}
		shown++
	}

	if dslURL := config.DetectServerFromDSL(); dslURL != "" && dslURL != cfg.DefaultServer {
		if _, ok := cfg.Servers[dslURL]; !ok {
			if shown > 0 {
				fmt.Println()
			}
			fmt.Printf("%s Commands in this directory target %s (from DSL settings)\n",
				output.Yellow("!"), output.Cyan(dslURL))
			printLoginHint(dslURL)
			shown++
		}
	}

	if shown == 0 {
		fmt.Println(output.Red("✗"), "Not logged in to any TeamCity server")
		fmt.Println("\nRun", output.Cyan("teamcity auth login"), "to authenticate")
		if config.IsBuildEnvironment() {
			fmt.Println("\n" + output.Yellow("!") + " Build environment detected but credentials not found in properties file")
		}
	}

	return nil
}

func sortedServerURLs(cfg *config.Config) []string {
	urls := slices.Collect(maps.Keys(cfg.Servers))
	slices.SortFunc(urls, func(a, b string) int {
		if ad, bd := a == cfg.DefaultServer, b == cfg.DefaultServer; ad != bd {
			if ad {
				return -1
			}
			return 1
		}
		return cmp.Compare(a, b)
	})
	return urls
}

// printLoginHint probes guest access on serverURL and prints a targeted suggestion.
func printLoginHint(serverURL string) {
	loginCmd := output.Cyan("teamcity auth login --server " + serverURL)
	if probeGuestAccess(serverURL) {
		fmt.Printf("  Run %s, or set %s for guest access\n", loginCmd, output.Cyan("TEAMCITY_GUEST=1"))
	} else {
		fmt.Printf("  Run %s to authenticate\n", loginCmd)
	}
}

// probeGuestAccess checks whether the server at serverURL supports guest access.
func probeGuestAccess(serverURL string) bool {
	if serverURL == "" {
		return false
	}
	guest := api.NewGuestClient(serverURL, api.WithDebugFunc(output.Debug))
	_, err := guest.GetServer()
	return err == nil
}

// notAuthenticatedError returns a not-authenticated error with a hint that
// conditionally includes the guest access suggestion based on server support.
func notAuthenticatedError(serverURL string) *tcerrors.UserError {
	err := tcerrors.NotAuthenticated()
	if probeGuestAccess(serverURL) {
		err.Suggestion += ", or set TEAMCITY_GUEST=1 for guest access"
	}
	return err
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

func showExplicitAuthStatus(serverURL, token, tokenSource, suffix string) {
	warnInsecureHTTP(serverURL, "authentication token")
	client := api.NewClient(serverURL, token, api.WithDebugFunc(output.Debug))
	user, err := client.GetCurrentUser()
	if err != nil {
		fmt.Printf("%s Server: %s%s\n", output.Red("✗"), serverURL, suffix)
		fmt.Println("  Token is invalid or expired")
		return
	}

	fmt.Printf("%s Logged in to %s%s\n", output.Green("✓"), output.Cyan(serverURL), suffix)
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
}

func showGuestAuthStatus(serverURL, suffix string) {
	client := api.NewGuestClient(serverURL, api.WithDebugFunc(output.Debug))
	server, err := client.GetServer()
	if err != nil {
		fmt.Printf("%s Server: %s%s\n", output.Red("✗"), serverURL, suffix)
		fmt.Println("  Guest access is not available")
		return
	}

	fmt.Printf("%s Guest access to %s%s\n", output.Green("✓"), output.Cyan(serverURL), suffix)
	fmt.Printf("  Server: TeamCity %d.%d (build %s)\n", server.VersionMajor, server.VersionMinor, server.BuildNumber)

	if err := client.CheckVersion(); err != nil {
		fmt.Printf("  %s %s\n", output.Yellow("!"), err.Error())
	} else {
		fmt.Printf("  %s API compatible\n", output.Green("✓"))
	}
}

func showBuildAuthStatus(buildAuth *config.BuildAuth) {
	warnInsecureHTTP(buildAuth.ServerURL, "credentials")
	client := api.NewClientWithBasicAuth(buildAuth.ServerURL, buildAuth.Username, buildAuth.Password, api.WithDebugFunc(output.Debug))
	server, err := client.GetServer()
	if err != nil {
		fmt.Printf("%s Server: %s\n", output.Red("✗"), buildAuth.ServerURL)
		fmt.Println("  Build credentials are invalid")
		return
	}

	fmt.Printf("%s Connected to %s\n", output.Green("✓"), output.Cyan(buildAuth.ServerURL))
	fmt.Printf("  Auth: %s\n", output.Faint("Build-level credentials"))
	fmt.Printf("  Scope: %s\n", output.Faint("Build-level access"))
	fmt.Printf("  Server: TeamCity %d.%d (build %s)\n", server.VersionMajor, server.VersionMinor, server.BuildNumber)
}
