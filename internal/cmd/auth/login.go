package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/charmbracelet/huh"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

type loginOpts struct {
	serverURL       string
	token           string
	guest           bool
	insecureStorage bool
	noBrowser       bool
}

func newAuthLoginCmd(f *cmdutil.Factory) *cobra.Command {
	var opts loginOpts

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a TeamCity server",
		Long: `Authenticate with a TeamCity server using an access token.

The token is stored in the system keyring (macOS Keychain, GNOME Keyring,
Windows Credential Manager). Use --insecure-storage to fall back to plain
text in the config file.

For CI/CD, set TEAMCITY_URL and TEAMCITY_TOKEN environment variables
(or TEAMCITY_URL + TEAMCITY_GUEST=1 for guest access).`,
		Example: `  # Interactive login with auto-discovered browser-based auth
  teamcity auth login

  # Skip browser-based auth, enter a token manually
  teamcity auth login -s https://teamcity.example.com --no-browser

  # Guest access (read-only, if enabled on the server)
  teamcity auth login -s https://teamcity.example.com --guest`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogin(f, &opts)
		},
	}

	cmd.Flags().StringVarP(&opts.serverURL, "server", "s", "", "TeamCity server URL")
	cmd.Flags().StringVarP(&opts.token, "token", "t", "", "Access token")
	cmd.Flags().BoolVar(&opts.guest, "guest", false, "Use guest authentication (no token needed; must be enabled on the server)")
	cmd.Flags().BoolVar(&opts.insecureStorage, "insecure-storage", false, "Store token in plain text config file instead of system keyring")
	cmd.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "Skip browser-based auth, use manual token entry")

	return cmd
}

func runAuthLogin(f *cmdutil.Factory, opts *loginOpts) error {
	if opts.guest && opts.token != "" {
		return api.Validation(
			"cannot use --guest with --token",
			"Use either --guest for guest access or --token for token authentication",
		)
	}

	p := f.Printer
	ctx := f.Context()
	interactive := f.IsInteractive()

	if interactive && (opts.serverURL == "" || (!opts.guest && opts.token == "")) {
		p.Tip(output.TipCancelAnytime)
		_, _ = fmt.Fprintln(p.Out)
	}

	serverURL, err := resolveServerURL(ctx, p, opts.serverURL, interactive)
	if err != nil {
		return err
	}

	reason := "authentication token"
	if opts.guest {
		reason = "guest access"
	}
	f.WarnInsecureHTTP(serverURL, reason)

	if opts.guest {
		return finishGuestLogin(ctx, f, serverURL)
	}
	return finishTokenLogin(ctx, f, serverURL, opts, interactive)
}

// resolveServerURL prompts and probes in a loop until a reachable TeamCity server is found or the user cancels.
func resolveServerURL(ctx context.Context, p *output.Printer, initial string, interactive bool) (string, error) {
	serverURL := initial
	detected := ""
	savedDefault := ""
	if serverURL == "" {
		detected = config.DetectServerFromDSL()
		if detected == "" {
			savedDefault = config.Get().DefaultServer
		}
	}
	for {
		if serverURL == "" {
			if !interactive {
				if detected == "" {
					return "", api.RequiredFlag("server")
				}
				serverURL = detected
			} else {
				switch {
				case detected != "":
					_, _ = fmt.Fprintf(p.Out, "%s Detected server from %s/pom.xml\n",
						output.Green("✓"), config.DetectTeamCityDir())
					serverURL = detected
					detected = ""
				case savedDefault != "":
					serverURL = savedDefault
					savedDefault = ""
				}
				if err := cmdutil.Prompt(huh.NewInput().
					Title("TeamCity server URL").
					Description("e.g., https://teamcity.example.com").
					Validate(cmdutil.RequireNonEmpty).
					Value(&serverURL)); err != nil {
					return "", err
				}
			}
		}

		serverURL = config.NormalizeURL(serverURL)

		p.Progress("Checking %s... ", output.Cyan(serverURL))
		if err := api.ProbeTeamCity(ctx, serverURL); err != nil {
			p.Info("%s", output.Red("✗"))
			if errors.Is(err, context.Canceled) {
				return "", err
			}
			friendly := friendlyError(err)
			if !interactive {
				return "", fmt.Errorf("cannot reach TeamCity at %s: %s", serverURL, friendly)
			}
			p.Warn("%s", friendly)
			serverURL = ""
			continue
		}
		p.Info("%s", output.Green("✓"))
		return serverURL, nil
	}
}

func finishGuestLogin(ctx context.Context, f *cmdutil.Factory, serverURL string) error {
	p := f.Printer
	p.Progress("Validating guest access... ")

	client := api.NewGuestClient(serverURL,
		api.WithDebugFunc(p.Debug),
		api.WithVersion(version.String()),
	).WithContext(ctx)
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
	_, _ = fmt.Fprintf(p.Out, "  Server: TeamCity %d.%d (build %s)\n",
		server.VersionMajor, server.VersionMinor, server.BuildNumber)
	return nil
}

func finishTokenLogin(ctx context.Context, f *cmdutil.Factory, serverURL string, opts *loginOpts, interactive bool) error {
	p := f.Printer
	token := opts.token
	var tokenValidUntil string
	pkceTried := token == "" && !opts.noBrowser && interactive
	if pkceTried {
		token, tokenValidUntil = attemptPkceLogin(ctx, p, serverURL)
	}

	token, user, err := resolveToken(ctx, p, serverURL, token, pkceTried, interactive)
	if err != nil {
		return err
	}

	insecureFallback, err := config.SetServerWithKeyring(serverURL, token, user.Username, tokenValidUntil, opts.insecureStorage)
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	p.Success("Logged in to %s as %s", output.Cyan(serverURL), output.Cyan(user.Name))
	if insecureFallback {
		p.Warn("Token stored in plain text at %s", config.ConfigPath())
	} else {
		p.Success("Token stored in system keyring")
	}
	if tokenValidUntil != "" {
		if expiry, err := time.Parse(time.RFC3339, tokenValidUntil); err == nil {
			p.Info("Token expires: %s", output.Yellow(expiry.Local().Format("Jan 2, 2006")))
		}
	}
	if !config.IsReadOnly() {
		_, _ = fmt.Fprintln(p.Out)
		p.Tip("%s", output.TipEnableReadOnly())
	}
	return nil
}

// resolveToken prompts for a token and validates it, looping on invalid tokens when interactive.
func resolveToken(ctx context.Context, p *output.Printer, serverURL, initial string, pkceTried, interactive bool) (string, *api.User, error) {
	token := initial
	instructionsShown := false
	for {
		if token == "" {
			if !interactive {
				return "", nil, api.RequiredFlag("token")
			}
			if !instructionsShown {
				if err := printTokenInstructions(p, serverURL, pkceTried); err != nil {
					return "", nil, err
				}
				instructionsShown = true
			}
			if err := cmdutil.PromptSecret("Paste your access token", &token); err != nil {
				return "", nil, err
			}
		}

		p.Progress("Validating... ")
		client := api.NewClient(serverURL, token,
			api.WithDebugFunc(p.Debug),
			api.WithVersion(version.String()),
		).WithContext(ctx)
		user, err := client.GetCurrentUser()
		if err != nil {
			p.Info("%s", output.Red("✗"))
			if errors.Is(err, context.Canceled) {
				return "", nil, err
			}
			if !interactive {
				return "", nil, err
			}
			p.Warn("%s", friendlyError(err))
			token = ""
			continue
		}
		p.Info("%s", output.Green("✓"))
		return token, user, nil
	}
}

func printTokenInstructions(p *output.Printer, serverURL string, pkceTried bool) error {
	tokenURL := serverURL + "/profile.html?item=accessTokens"

	_, _ = fmt.Fprintln(p.Out)
	if pkceTried {
		p.Tip("Use --no-browser to skip browser login and enter a token manually")
		_, _ = fmt.Fprintln(p.Out)
	}
	_, _ = fmt.Fprintln(p.Out, output.Yellow("!"), "To authenticate, you need an access token.")
	_, _ = fmt.Fprintf(p.Out, "  Generate one at: %s\n", tokenURL)
	_, _ = fmt.Fprintln(p.Out)

	openBrowser := true
	if err := cmdutil.Confirm("Open browser to generate token?", &openBrowser); err != nil {
		return err
	}
	if !openBrowser {
		return nil
	}
	if err := browser.OpenURL(tokenURL); err != nil {
		_, _ = fmt.Fprintf(p.Out, "  Could not open browser. Please visit: %s\n", tokenURL)
	} else {
		_, _ = fmt.Fprintln(p.Out, output.Green("  ✓"), "Opened browser")
	}
	_, _ = fmt.Fprintln(p.Out)
	return nil
}

// friendlyError maps low-level probe and HTTP errors to short, actionable messages.
func friendlyError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "connection timed out"
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "no such host"):
		return "DNS lookup failed — check the hostname"
	case strings.Contains(s, "connection refused"):
		return "connection refused — is the server running?"
	case strings.Contains(s, "i/o timeout"):
		return "connection timed out"
	case strings.Contains(s, "x509"), strings.Contains(s, "certificate"):
		return "TLS certificate error — check the URL scheme and hostname"
	case strings.Contains(s, "401"), strings.Contains(strings.ToLower(s), "unauthorized"):
		return "token rejected — the token is invalid, expired, or lacks required permissions"
	}
	return s
}
