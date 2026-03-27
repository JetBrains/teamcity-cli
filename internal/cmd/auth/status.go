package auth

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

func newAuthStatusCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus(f)
		},
	}

	return cmd
}

func runAuthStatus(f *cmdutil.Factory) error {
	p := f.Printer

	if envURL := os.Getenv(config.EnvServerURL); envURL != "" {
		envURL = config.NormalizeURL(envURL)
		if config.IsGuestAuth() {
			showGuestAuthStatus(p, envURL, "")
			return nil
		}
		if envToken := os.Getenv(config.EnvToken); envToken != "" {
			showExplicitAuthStatus(f, envURL, envToken, "env", "")
			return nil
		}
	}

	if buildAuth, ok := config.GetBuildAuth(); ok {
		showBuildAuthStatus(f, buildAuth)
		return nil
	}

	cfg := config.Get()
	shown := 0

	urls := sortedServerURLs(cfg)
	for i, serverURL := range urls {
		if i > 0 {
			_, _ = fmt.Fprintln(p.Out)
		}
		sc := cfg.Servers[serverURL]
		suffix := ""
		if len(urls) > 1 && serverURL == cfg.DefaultServer {
			suffix = " (default)"
		}

		if sc.Guest {
			showGuestAuthStatus(p, serverURL, suffix)
		} else if token, src, krErr := config.GetTokenForServer(serverURL); token != "" {
			showExplicitAuthStatus(f, serverURL, token, src, suffix)
		} else {
			_, _ = fmt.Fprintf(p.Out, "%s %s%s\n", output.Red("✗"), serverURL, suffix)
			showCredentialsDiagnostic(p, serverURL, sc, krErr)
		}
		shown++
	}

	if dslURL := config.DetectServerFromDSL(); dslURL != "" && dslURL != cfg.DefaultServer {
		if _, ok := cfg.Servers[dslURL]; !ok {
			if shown > 0 {
				_, _ = fmt.Fprintln(p.Out)
			}
			_, _ = fmt.Fprintf(p.Out, "%s Commands in this directory target %s (from DSL settings)\n",
				output.Yellow("!"), output.Cyan(dslURL))
			printLoginHint(p, dslURL)
			shown++
		}
	}

	if shown == 0 {
		_, _ = fmt.Fprintln(p.Out, output.Red("✗"), "Not logged in to any TeamCity server")
		_, _ = fmt.Fprintln(p.Out, "\nRun", output.Cyan("teamcity auth login"), "to authenticate")
		if config.IsBuildEnvironment() {
			_, _ = fmt.Fprintln(p.Out, "\n"+output.Yellow("!")+" Build environment detected but credentials not found in properties file")
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

func showCredentialsDiagnostic(p *output.Printer, serverURL string, sc config.ServerConfig, krErr error) {
	if sc.User != "" && sc.Token == "" {
		if krErr != nil {
			_, _ = fmt.Fprintf(p.Out, "  Token is in the system keyring but could not be retrieved: %v\n", krErr)
		} else {
			_, _ = fmt.Fprintln(p.Out, "  Token was expected in the system keyring but is missing")
		}
	} else {
		_, _ = fmt.Fprintln(p.Out, "  Token is missing or could not be retrieved")
	}

	_, _ = fmt.Fprintf(p.Out, "  %s To authenticate in this environment:\n", output.Yellow("!"))
	_, _ = fmt.Fprintf(p.Out, "    • Set %s and %s environment variables\n",
		output.Cyan("TEAMCITY_URL"), output.Cyan("TEAMCITY_TOKEN"))
	_, _ = fmt.Fprintf(p.Out, "    • Or run %s\n",
		output.Cyan("teamcity auth login --server "+serverURL+" --insecure-storage"))
	if cmdutil.ProbeGuestAccess(serverURL) {
		_, _ = fmt.Fprintf(p.Out, "    • Or set %s for read-only guest access\n", output.Cyan("TEAMCITY_GUEST=1"))
	}
}

func printLoginHint(p *output.Printer, serverURL string) {
	loginCmd := output.Cyan("teamcity auth login --server " + serverURL)
	if cmdutil.ProbeGuestAccess(serverURL) {
		_, _ = fmt.Fprintf(p.Out, "  Run %s, or set %s for guest access\n", loginCmd, output.Cyan("TEAMCITY_GUEST=1"))
	} else {
		_, _ = fmt.Fprintf(p.Out, "  Run %s to authenticate\n", loginCmd)
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

func showExplicitAuthStatus(f *cmdutil.Factory, serverURL, token, tokenSource, suffix string) {
	p := f.Printer
	f.WarnInsecureHTTP(serverURL, "authentication token")
	client := api.NewClient(serverURL, token, api.WithDebugFunc(p.Debug))
	user, err := client.GetCurrentUser()
	if err != nil {
		_, _ = fmt.Fprintf(p.Out, "%s Server: %s%s\n", output.Red("✗"), serverURL, suffix)
		_, _ = fmt.Fprintln(p.Out, "  Token is invalid or expired")
		return
	}

	_, _ = fmt.Fprintf(p.Out, "%s Logged in to %s%s\n", output.Green("✓"), output.Cyan(serverURL), suffix)
	_, _ = fmt.Fprintf(p.Out, "  User: %s (%s) · %s\n", user.Name, user.Username, tokenSourceLabel(tokenSource))

	if expiry := config.GetTokenExpiry(); expiry != "" {
		if t, err := time.Parse(time.RFC3339, expiry); err == nil {
			remaining := time.Until(t)
			switch {
			case remaining <= 0:
				_, _ = fmt.Fprintf(p.Out, "  %s Token expired on %s\n", output.Red("✗"), t.Local().Format("Jan 2, 2006"))
				_, _ = fmt.Fprintf(p.Out, "  Run %s to re-authenticate\n", output.Cyan("teamcity auth login"))
			case remaining <= 3*24*time.Hour:
				_, _ = fmt.Fprintf(p.Out, "  %s Token expires %s (on %s)\n", output.Yellow("!"), output.Yellow(humanize.Time(t)), t.Local().Format("Jan 2, 2006"))
			default:
				_, _ = fmt.Fprintf(p.Out, "  Token expires: %s\n", t.Local().Format("Jan 2, 2006"))
			}
		}
	}

	server, err := client.ServerVersion()
	if err == nil {
		_, _ = fmt.Fprintf(p.Out, "  Server: TeamCity %d.%d (build %s)\n", server.VersionMajor, server.VersionMinor, server.BuildNumber)

		if err := client.CheckVersion(); err != nil {
			_, _ = fmt.Fprintf(p.Out, "  %s %s\n", output.Yellow("!"), err.Error())
		} else {
			_, _ = fmt.Fprintf(p.Out, "  %s API compatible\n", output.Green("✓"))
		}
	}
}

func showGuestAuthStatus(p *output.Printer, serverURL, suffix string) {
	client := api.NewGuestClient(serverURL, api.WithDebugFunc(p.Debug))
	server, err := client.GetServer()
	if err != nil {
		_, _ = fmt.Fprintf(p.Out, "%s Server: %s%s\n", output.Red("✗"), serverURL, suffix)
		_, _ = fmt.Fprintln(p.Out, "  Guest access is not available")
		return
	}

	_, _ = fmt.Fprintf(p.Out, "%s Guest access to %s%s\n", output.Green("✓"), output.Cyan(serverURL), suffix)
	_, _ = fmt.Fprintf(p.Out, "  Server: TeamCity %d.%d (build %s)\n", server.VersionMajor, server.VersionMinor, server.BuildNumber)

	if err := client.CheckVersion(); err != nil {
		_, _ = fmt.Fprintf(p.Out, "  %s %s\n", output.Yellow("!"), err.Error())
	} else {
		_, _ = fmt.Fprintf(p.Out, "  %s API compatible\n", output.Green("✓"))
	}
}

func showBuildAuthStatus(f *cmdutil.Factory, buildAuth *config.BuildAuth) {
	p := f.Printer
	f.WarnInsecureHTTP(buildAuth.ServerURL, "credentials")
	client := api.NewClientWithBasicAuth(buildAuth.ServerURL, buildAuth.Username, buildAuth.Password, api.WithDebugFunc(p.Debug))
	server, err := client.GetServer()
	if err != nil {
		_, _ = fmt.Fprintf(p.Out, "%s Server: %s\n", output.Red("✗"), buildAuth.ServerURL)
		_, _ = fmt.Fprintln(p.Out, "  Build credentials are invalid")
		return
	}

	_, _ = fmt.Fprintf(p.Out, "%s Connected to %s\n", output.Green("✓"), output.Cyan(buildAuth.ServerURL))
	_, _ = fmt.Fprintf(p.Out, "  Auth: %s\n", output.Faint("Build-level credentials"))
	_, _ = fmt.Fprintf(p.Out, "  Scope: %s\n", output.Faint("Build-level access"))
	_, _ = fmt.Fprintf(p.Out, "  Server: TeamCity %d.%d (build %s)\n", server.VersionMajor, server.VersionMinor, server.BuildNumber)
}
