package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const authCodeLifetime = 5 * time.Minute

func newAuthLoginCmd(f *cmdutil.Factory) *cobra.Command {
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
2. Automatically authenticate via browser with PKCE when browser-based login is available
3. Or use manual token entry to generate and paste an access token
4. Validate and store the token securely

The token is stored in your system keyring (macOS Keychain, GNOME Keyring,
Windows Credential Manager) when available. Use --insecure-storage to store
the token in plain text in the config file instead.

To skip browser-based login and use manual token entry:
  teamcity auth login -s https://teamcity.example.com --no-browser

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
				return runAuthLoginGuest(f, serverURL, token)
			}
			return runAuthLogin(f, serverURL, token, insecureStorage, noBrowser)
		},
	}

	cmd.Flags().StringVarP(&serverURL, "server", "s", "", "TeamCity server URL")
	cmd.Flags().StringVarP(&token, "token", "t", "", "Access token")
	cmd.Flags().BoolVar(&guest, "guest", false, "Use guest authentication (no token needed, must be enabled on the server)")
	cmd.Flags().BoolVar(&insecureStorage, "insecure-storage", false, "Store token in plain text config file instead of system keyring")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Skip browser-based auth, use manual token entry")

	return cmd
}

func runAuthLogin(f *cmdutil.Factory, serverURL, token string, insecureStorage bool, noBrowser bool) error {
	isInteractive := f.IsInteractive()

	p := f.Printer

	if serverURL == "" {
		detectedServer := config.DetectServerFromDSL()

		if !isInteractive {
			if detectedServer != "" {
				serverURL = detectedServer
			} else {
				return api.RequiredFlag("server")
			}
		} else {
			prompt := &survey.Input{
				Message: "TeamCity server URL:",
				Help:    "e.g., https://teamcity.example.com",
			}

			if detectedServer != "" {
				prompt.Default = detectedServer
				dslDir := config.DetectTeamCityDir()
				_, _ = fmt.Fprintf(p.Out, "%s Detected server from %s/pom.xml\n", output.Green("✓"), dslDir)
			}

			if err := survey.AskOne(prompt, &serverURL, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
		}
	}

	serverURL = config.NormalizeURL(serverURL)

	var tokenValidUntil string
	pkceChecked := false
	if token == "" && !noBrowser && isInteractive {
		pkceChecked = true
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		enabled, _ := api.IsPkceEnabled(ctx, serverURL)
		cancel()
		if enabled {
			p.Info("Secure browser login available on this server")
			if tokenResp, err := runPkceLogin(p, serverURL); err != nil {
				p.Warn("Browser auth failed: %v", err)
				p.Info("Falling back to manual token entry...")
			} else {
				token = tokenResp.AccessToken
				tokenValidUntil = tokenResp.ValidUntil
			}
		}
	}

	if token == "" {
		if !isInteractive {
			return api.RequiredFlag("token")
		}

		tokenURL := fmt.Sprintf("%s/profile.html?item=accessTokens", serverURL)

		_, _ = fmt.Fprintln(p.Out)
		if pkceChecked {
			_, _ = fmt.Fprintln(p.Out, output.Faint("Tip: Use --no-browser to skip browser login and enter a token manually"))
			_, _ = fmt.Fprintln(p.Out)
		}
		_, _ = fmt.Fprintln(p.Out, output.Yellow("!"), "To authenticate, you need an access token.")
		_, _ = fmt.Fprintf(p.Out, "  Generate one at: %s\n", tokenURL)
		_, _ = fmt.Fprintln(p.Out)

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
				_, _ = fmt.Fprintf(p.Out, "  Could not open browser. Please visit: %s\n", tokenURL)
			} else {
				_, _ = fmt.Fprintln(p.Out, output.Green("  ✓"), "Opened browser")
			}
			_, _ = fmt.Fprintln(p.Out)
		}

		tokenPrompt := &survey.Password{
			Message: "Paste your access token:",
		}
		if err := survey.AskOne(tokenPrompt, &token, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	f.WarnInsecureHTTP(serverURL, "authentication token")
	p.Infof("Validating... ")

	client := api.NewClient(serverURL, token, api.WithDebugFunc(p.Debug), api.WithVersion(version.String()))
	user, err := client.GetCurrentUser()
	if err != nil {
		p.Info("%s", output.Red("✗"))
		p.Debug("validation error: %v", err)
		return err
	}

	p.Info("%s", output.Green("✓"))

	insecureFallback, err := config.SetServerWithKeyring(serverURL, token, user.Username, tokenValidUntil, insecureStorage)
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	p.Success("Logged in as %s", output.Cyan(user.Name))
	if insecureFallback {
		_, _ = fmt.Fprintf(p.Out, "%s Token stored in plain text at %s\n", output.Yellow("!"), config.ConfigPath())
	} else {
		_, _ = fmt.Fprintf(p.Out, "%s Token stored in system keyring\n", output.Green("✓"))
	}
	if tokenValidUntil != "" {
		if expiry, err := time.Parse(time.RFC3339, tokenValidUntil); err == nil {
			p.Info("Token expires: %s", output.Yellow(expiry.Local().Format("Jan 2, 2006")))
		}
	}

	return nil
}

func runAuthLoginGuest(f *cmdutil.Factory, serverURL, token string) error {
	if token != "" {
		return api.Validation(
			"cannot use --guest with --token",
			"Use either --guest for guest access or --token for token authentication",
		)
	}

	isInteractive := f.IsInteractive()

	if serverURL == "" {
		if !isInteractive {
			return api.RequiredFlag("server")
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

	p := f.Printer
	f.WarnInsecureHTTP(serverURL, "guest access")
	p.Infof("Validating guest access... ")

	client := api.NewGuestClient(serverURL, api.WithDebugFunc(p.Debug), api.WithVersion(version.String()))
	server, err := client.GetServer()
	if err != nil {
		p.Info("%s", output.Red("✗"))
		return api.Validation(
			"Guest access validation failed",
			"Verify the server URL and that guest access is enabled on the server",
		)
	}

	p.Info("%s", output.Green("✓"))

	if err := config.SetGuestServer(serverURL); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	p.Success("Guest access to %s", output.Cyan(serverURL))
	_, _ = fmt.Fprintf(p.Out, "  Server: TeamCity %d.%d (build %s)\n", server.VersionMajor, server.VersionMinor, server.BuildNumber)

	return nil
}

func runPkceLogin(p *output.Printer, serverURL string) (*api.TokenResponse, error) {
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

	if err := browser.OpenURL(authURL); err != nil {
		p.Warn("Could not open browser automatically: %v", err)
		_, _ = fmt.Fprintf(p.Out, "\nOpen this URL in your browser to authenticate:\n  %s\n\n", authURL)
	} else {
		p.Info("Opening browser for authentication...")
	}
	_, _ = fmt.Fprintf(p.Out, "  %s Approve access in TeamCity\n", output.Yellow("→"))

	select {
	case result := <-callbackServer.ResultChan:
		if result.Error != "" {
			return nil, fmt.Errorf("authorization denied: %s", result.Error)
		}
		if subtle.ConstantTimeCompare([]byte(result.State), []byte(state)) != 1 {
			return nil, fmt.Errorf("state mismatch: possible CSRF attack")
		}
		_, _ = fmt.Fprintln(p.Out)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return api.ExchangeCodeForToken(ctx, serverURL, result.Code, verifier, redirectURI)

	case <-time.After(authCodeLifetime):
		return nil, fmt.Errorf("timeout waiting for callback (exceeded %v)", authCodeLifetime)
	}
}
