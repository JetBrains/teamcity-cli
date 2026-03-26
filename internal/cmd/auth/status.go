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

func showCredentialsDiagnostic(serverURL string, sc config.ServerConfig, krErr error) {
	if sc.User != "" && sc.Token == "" {
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
	if cmdutil.ProbeGuestAccess(serverURL) {
		fmt.Printf("    • Or set %s for read-only guest access\n", output.Cyan("TEAMCITY_GUEST=1"))
	}
}

func printLoginHint(serverURL string) {
	loginCmd := output.Cyan("teamcity auth login --server " + serverURL)
	if cmdutil.ProbeGuestAccess(serverURL) {
		fmt.Printf("  Run %s, or set %s for guest access\n", loginCmd, output.Cyan("TEAMCITY_GUEST=1"))
	} else {
		fmt.Printf("  Run %s to authenticate\n", loginCmd)
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
	cmdutil.WarnInsecureHTTP(serverURL, "authentication token")
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
	cmdutil.WarnInsecureHTTP(buildAuth.ServerURL, "credentials")
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
