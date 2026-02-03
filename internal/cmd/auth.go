package cmd

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

// authCodeLifetime is the maximum time to wait for the OAuth callback
const authCodeLifetime = 5 * time.Minute

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
	var noBrowser bool
	var scopes []string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a TeamCity server",
		Long: `Authenticate with a TeamCity server using an access token.

This will:
1. Prompt for your TeamCity server URL
2. Automatically authenticate via browser (if PKCE is enabled)
3. Or open your browser to generate an access token manually
4. Validate and store the token securely

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
			return runAuthLogin(serverURL, token, insecureStorage, noBrowser, scopes)
		},
	}

	cmd.Flags().StringVarP(&serverURL, "server", "s", "", "TeamCity server URL")
	cmd.Flags().StringVarP(&token, "token", "t", "", "Access token")
	cmd.Flags().BoolVar(&guest, "guest", false, "Use guest authentication (no token needed, must be enabled on the server)")
	cmd.Flags().BoolVar(&insecureStorage, "insecure-storage", false, "Store token in plain text config file instead of system keyring")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Skip browser-based auth, use manual token entry")
	cmd.Flags().StringSliceVar(&scopes, "scopes", api.DefaultScopes(), "Permissions for the token (PKCE only)")

	return cmd
}

func runAuthLogin(serverURL, token string, insecureStorage bool, noBrowser bool, scopes []string) error {
	isInteractive := !NoInput && output.IsStdinTerminal()

	if serverURL == "" {
		// Try to detect server from DSL (pom.xml)
		detectedServer := config.DetectServerFromDSL()

		if !isInteractive {
			if detectedServer != "" {
				serverURL = detectedServer
			} else {
				return tcerrors.RequiredFlag("server")
			}
		} else {
			// Interactive mode: let user confirm or change detected server
			prompt := &survey.Input{
				Message: "TeamCity server URL:",
				Help:    "e.g., https://teamcity.example.com",
			}

			if detectedServer != "" {
				prompt.Default = detectedServer
				dslDir := config.DetectTeamCityDir()
				fmt.Printf("%s Detected server from %s/pom.xml\n", output.Green("✓"), dslDir)
			}

			if err := survey.AskOne(prompt, &serverURL, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
		}
	}

	serverURL = config.NormalizeURL(serverURL)

	// Try PKCE authentication first (if available and allowed)
	var tokenValidUntil string
	pkceChecked := false
	if token == "" && !noBrowser && isInteractive {
		pkceChecked = true
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		enabled, _ := api.IsPkceEnabled(ctx, serverURL)
		cancel()
		if enabled {
			output.Info("Secure browser login enabled on this server")
			if tokenResp, err := runPkceLogin(serverURL, scopes); err != nil {
				output.Warn("Browser auth failed: %v", err)
				output.Info("Falling back to manual token entry...")
			} else {
				token = tokenResp.AccessToken
				tokenValidUntil = tokenResp.ValidUntil
			}
		}
	}

	// Fall back to manual token entry
	if token == "" {
		if !isInteractive {
			return tcerrors.RequiredFlag("token")
		}

		tokenURL := fmt.Sprintf("%s/profile.html?item=accessTokens", serverURL)

		fmt.Println()
		if pkceChecked {
			fmt.Println(output.Faint("Tip: Server admins can enable secure browser login for easier authentication"))
			fmt.Println()
		}
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
	if tokenValidUntil != "" {
		if expiry, err := time.Parse(time.RFC3339, tokenValidUntil); err == nil {
			output.Info("Token expires: %s", output.Yellow(expiry.Local().Format("Jan 2, 2006")))
		}
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

func runPkceLogin(serverURL string, scopes []string) (*api.TokenResponse, error) {
	verifier, err := api.GenerateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("generate code verifier: %w", err)
	}
	state, err := api.GenerateState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	listener, port, err := api.FindAvailableListener()
	if err != nil {
		return nil, fmt.Errorf("find available port: %w", err)
	}

	callbackServer := api.NewCallbackServer(listener, port)
	callbackServer.Start()
	defer callbackServer.Shutdown()

	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, api.DefaultCallbackPath)
	authURL := api.BuildAuthorizeURL(serverURL, redirectURI, api.GenerateCodeChallenge(verifier), state, scopes)

	output.Info("Opening browser for authentication...")
	fmt.Printf("  %s Approve access in TeamCity\n", output.Yellow("→"))

	if err := browser.OpenURL(authURL); err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	select {
	case result := <-callbackServer.ResultChan:
		if result.Error != "" {
			return nil, fmt.Errorf("authorization denied: %s", result.Error)
		}
		if subtle.ConstantTimeCompare([]byte(result.State), []byte(state)) != 1 {
			return nil, fmt.Errorf("state mismatch: possible CSRF attack")
		}
		fmt.Println()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return api.ExchangeCodeForToken(ctx, serverURL, result.Code, verifier, redirectURI)

	case <-time.After(authCodeLifetime):
		return nil, fmt.Errorf("timeout waiting for callback (exceeded %v)", authCodeLifetime)
	}
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

	if config.IsGuestAuth() && serverURL != "" {
		return showGuestAuthStatus(serverURL)
	}

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

func showGuestAuthStatus(serverURL string) error {
	client := api.NewGuestClient(serverURL, api.WithDebugFunc(output.Debug))
	server, err := client.GetServer()
	if err != nil {
		fmt.Printf("%s Server: %s\n", output.Red("✗"), serverURL)
		fmt.Println("  Guest access is not available")
		return nil
	}

	fmt.Printf("%s Guest access to %s\n", output.Green("✓"), output.Cyan(serverURL))
	fmt.Printf("  Server: TeamCity %d.%d (build %s)\n", server.VersionMajor, server.VersionMinor, server.BuildNumber)

	if err := client.CheckVersion(); err != nil {
		fmt.Printf("  %s %s\n", output.Yellow("!"), err.Error())
	} else {
		fmt.Printf("  %s API compatible\n", output.Green("✓"))
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
