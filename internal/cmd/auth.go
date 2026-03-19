package cmd

import (
	"cmp"
	"context"
	"crypto/subtle"
	"fmt"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/dustin/go-humanize"
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
			return runAuthLogin(serverURL, token, insecureStorage, noBrowser)
		},
	}

	cmd.Flags().StringVarP(&serverURL, "server", "s", "", "TeamCity server URL")
	cmd.Flags().StringVarP(&token, "token", "t", "", "Access token")
	cmd.Flags().BoolVar(&guest, "guest", false, "Use guest authentication (no token needed, must be enabled on the server)")
	cmd.Flags().BoolVar(&insecureStorage, "insecure-storage", false, "Store token in plain text config file instead of system keyring")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Skip browser-based auth, use manual token entry")

	return cmd
}

func runAuthLogin(serverURL, token string, insecureStorage bool, noBrowser bool) error {
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
			if tokenResp, err := runPkceLogin(serverURL); err != nil {
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

	insecureFallback, err := config.SetServerWithKeyring(serverURL, token, user.Username, tokenValidUntil, insecureStorage)
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

func runPkceLogin(serverURL string) (*api.TokenResponse, error) {
	verifier, err := api.GenerateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("generate code verifier: %w", err)
	}
	state, err := api.GenerateState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	listener, err := api.FindAvailableListener()
	if err != nil {
		return nil, fmt.Errorf("find available port: %w", err)
	}

	callbackServer := api.NewCallbackServer(listener)
	callbackServer.Start()
	defer callbackServer.Shutdown()

	redirectURI := fmt.Sprintf("http://localhost:%d%s", callbackServer.Port, api.DefaultCallbackPath)
	authURL := api.BuildAuthorizeURL(serverURL, redirectURI, api.GenerateCodeChallenge(verifier), state, api.DefaultScopes())

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
		} else if token, src, krErr := config.GetTokenForServer(serverURL); token != "" {
			showExplicitAuthStatus(serverURL, token, src, suffix)
		} else {
			fmt.Printf("%s %s%s\n", output.Red("✗"), serverURL, suffix)
			showCredentialsDiagnostic(serverURL, sc, krErr)
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

// showCredentialsDiagnostic prints why credentials could not be retrieved for a
// server and offers clear remediation steps covering all auth methods.
func showCredentialsDiagnostic(serverURL string, sc config.ServerConfig, krErr error) {
	if sc.User != "" && sc.Token == "" {
		// Token was stored in keyring (User set, no plaintext token in config)
		if krErr != nil {
			fmt.Printf("  Token is in the system keyring but could not be retrieved: %v\n", krErr)
		} else {
			fmt.Println("  Token was expected in the system keyring but is missing")
		}
	} else {
		fmt.Println("  Token is missing or could not be retrieved")
	}

	fmt.Printf("  %s To authenticate in this environment:\n", output.Yellow("!"))
	fmt.Printf("    • Set %s and %s environment variables\n",
		output.Cyan("TEAMCITY_URL"), output.Cyan("TEAMCITY_TOKEN"))
	fmt.Printf("    • Or run %s\n",
		output.Cyan("teamcity auth login --server "+serverURL+" --insecure-storage"))
	if probeGuestAccess(serverURL) {
		fmt.Printf("    • Or set %s for read-only guest access\n", output.Cyan("TEAMCITY_GUEST=1"))
	}
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

// notAuthenticatedError returns a not-authenticated error with a hint that covers
// all authentication methods: environment variables, interactive login, and guest access.
func notAuthenticatedError(serverURL string, keyringErr error) *tcerrors.UserError {
	msg := "Not authenticated"
	if keyringErr != nil {
		msg = fmt.Sprintf("Not authenticated (could not access system keyring: %v)", keyringErr)
	}

	suggestion := "Set TEAMCITY_URL and TEAMCITY_TOKEN environment variables, or run 'teamcity auth login --insecure-storage'"
	if probeGuestAccess(serverURL) {
		suggestion += ", or set TEAMCITY_GUEST=1 for guest access"
	}

	return &tcerrors.UserError{
		Message:    msg,
		Suggestion: suggestion,
	}
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

	if expiry := config.GetTokenExpiry(); expiry != "" {
		if t, err := time.Parse(time.RFC3339, expiry); err == nil {
			remaining := time.Until(t)
			switch {
			case remaining <= 0:
				fmt.Printf("  %s Token expired on %s\n", output.Red("✗"), t.Local().Format("Jan 2, 2006"))
				fmt.Printf("  Run %s to re-authenticate\n", output.Cyan("teamcity auth login"))
			case remaining <= 3*24*time.Hour:
				fmt.Printf("  %s Token expires %s (on %s)\n", output.Yellow("!"), output.Yellow(humanize.Time(t)), t.Local().Format("Jan 2, 2006"))
			default:
				fmt.Printf("  Token expires: %s\n", t.Local().Format("Jan 2, 2006"))
			}
		}
	}

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
