package auth

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

type authStatusOptions struct {
	json bool
}

type authStatus struct {
	Server      string      `json:"server"`
	AuthMethod  string      `json:"auth_method"`
	TokenSource string      `json:"token_source,omitempty"`
	User        *authUser   `json:"user,omitempty"`
	ServerInfo  *serverInfo `json:"server_info,omitempty"`
	TokenExpiry string      `json:"token_expiry,omitempty"`
	Status      string      `json:"status"`
	Error       string      `json:"error,omitempty"`
	IsDefault   bool        `json:"is_default,omitempty"`

	versionCheckErr string `json:"-"`
	keyringErr      error  `json:"-"`
	configUser      string `json:"-"`
}

type authUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type serverInfo struct {
	VersionMajor int    `json:"version_major"`
	VersionMinor int    `json:"version_minor"`
	BuildNumber  string `json:"build_number"`
}

func newAuthStatusCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &authStatusOptions{}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		Example: `  teamcity auth status
  teamcity auth status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus(f, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runAuthStatus(f *cmdutil.Factory, opts *authStatusOptions) error {
	results := collectAuthStatuses(f)
	if opts.json {
		if len(results) == 0 {
			results = []authStatus{{Status: "error", Error: "not logged in to any TeamCity server"}}
		}
		return f.Printer.PrintJSON(results)
	}
	return renderAuthStatusHuman(f, results)
}

func collectAuthStatuses(f *cmdutil.Factory) []authStatus {
	if envURL := os.Getenv(config.EnvServerURL); envURL != "" {
		envURL = config.NormalizeURL(envURL)
		if config.IsGuestAuth() {
			return []authStatus{collectGuestStatus(f, envURL, false)}
		}
		if envToken := os.Getenv(config.EnvToken); envToken != "" {
			return []authStatus{collectTokenStatus(f, envURL, envToken, "env", false)}
		}
	}

	if buildAuth, ok := config.GetBuildAuth(); ok {
		return []authStatus{collectBuildStatus(f, buildAuth)}
	}

	cfg := config.Get()
	urls := sortedServerURLs(cfg)
	var results []authStatus
	for _, serverURL := range urls {
		sc := cfg.Servers[serverURL]
		isDefault := len(urls) > 1 && serverURL == cfg.DefaultServer

		if sc.Guest {
			results = append(results, collectGuestStatus(f, serverURL, isDefault))
			continue
		}

		token, src, krErr := config.GetTokenForServer(serverURL)
		if token != "" {
			results = append(results, collectTokenStatus(f, serverURL, token, src, isDefault))
		} else {
			results = append(results, authStatus{
				Server:     serverURL,
				Status:     "error",
				Error:      "token missing or could not be retrieved",
				IsDefault:  isDefault,
				keyringErr: krErr,
				configUser: sc.User,
			})
		}
	}
	return results
}

func collectGuestStatus(f *cmdutil.Factory, serverURL string, isDefault bool) authStatus {
	s := authStatus{Server: serverURL, AuthMethod: "guest", IsDefault: isDefault}
	client := api.NewGuestClient(serverURL, api.WithDebugFunc(f.Printer.Debug), api.WithVersion(version.String()))
	server, err := client.GetServer()
	if err != nil {
		s.Status = "error"
		s.Error = "guest access is not available"
		return s
	}
	s.Status = "guest"
	s.ServerInfo = &serverInfo{server.VersionMajor, server.VersionMinor, server.BuildNumber}
	if err := client.CheckVersion(); err != nil {
		s.versionCheckErr = err.Error()
	}
	return s
}

func collectTokenStatus(f *cmdutil.Factory, serverURL, token, tokenSource string, isDefault bool) authStatus {
	s := authStatus{Server: serverURL, AuthMethod: "token", TokenSource: tokenSource, IsDefault: isDefault}
	client := api.NewClient(serverURL, token, api.WithDebugFunc(f.Printer.Debug), api.WithVersion(version.String()))
	user, err := client.GetCurrentUser()
	if err != nil {
		s.Status = "error"
		if netErr, ok := errors.AsType[*api.NetworkError](err); ok {
			if api.IsSandboxBlockedError(netErr) {
				s.Error = "network access blocked by sandbox"
			} else {
				s.Error = netErr.Error()
			}
		} else {
			s.Error = "Token is invalid or expired"
		}
		return s
	}
	s.Status = "authenticated"
	s.User = &authUser{ID: user.ID, Username: user.Username, Name: user.Name}

	cfg := config.Get()
	if sc, ok := cfg.Servers[serverURL]; ok && sc.TokenExpiry != "" {
		s.TokenExpiry = sc.TokenExpiry
	} else if expiry := config.GetTokenExpiry(); expiry != "" {
		s.TokenExpiry = expiry
	}

	if server, err := client.ServerVersion(); err == nil {
		s.ServerInfo = &serverInfo{server.VersionMajor, server.VersionMinor, server.BuildNumber}
		if err := client.CheckVersion(); err != nil {
			s.versionCheckErr = err.Error()
		}
	}
	return s
}

func collectBuildStatus(f *cmdutil.Factory, buildAuth *config.BuildAuth) authStatus {
	s := authStatus{Server: buildAuth.ServerURL, AuthMethod: "build"}
	client := api.NewClientWithBasicAuth(buildAuth.ServerURL, buildAuth.Username, buildAuth.Password, api.WithDebugFunc(f.Printer.Debug), api.WithVersion(version.String()))
	server, err := client.GetServer()
	if err != nil {
		s.Status = "error"
		s.Error = "build credentials are invalid"
		return s
	}
	s.Status = "authenticated"
	s.ServerInfo = &serverInfo{server.VersionMajor, server.VersionMinor, server.BuildNumber}
	return s
}

func renderAuthStatusHuman(f *cmdutil.Factory, results []authStatus) error {
	p := f.Printer

	if len(results) == 0 {
		_, _ = fmt.Fprintln(p.Out, output.Red("✗"), "Not logged in to any TeamCity server")
		_, _ = fmt.Fprintln(p.Out, "\nRun", output.Cyan("teamcity auth login"), "to authenticate")
		if config.IsBuildEnvironment() {
			_, _ = fmt.Fprintln(p.Out, "\n"+output.Yellow("!")+" Build environment detected but credentials not found in properties file")
		}
		renderDSLHint(p, results)
		return nil
	}

	for i, s := range results {
		if i > 0 {
			_, _ = fmt.Fprintln(p.Out)
		}
		renderOneStatus(f, p, s)
	}

	renderDSLHint(p, results)
	return nil
}

func renderOneStatus(f *cmdutil.Factory, p *output.Printer, s authStatus) {
	suffix := ""
	if s.IsDefault {
		suffix = " (default)"
	}

	switch s.AuthMethod {
	case "token":
		f.WarnInsecureHTTP(s.Server, "authentication token")
	case "build":
		f.WarnInsecureHTTP(s.Server, "credentials")
	}

	switch {
	case s.Status == "guest":
		_, _ = fmt.Fprintf(p.Out, "%s Guest access to %s%s\n", output.Green("✓"), output.Cyan(s.Server), suffix)
		renderServerInfo(p, s)

	case s.Status == "authenticated" && s.AuthMethod == "build":
		_, _ = fmt.Fprintf(p.Out, "%s Connected to %s\n", output.Green("✓"), output.Cyan(s.Server))
		_, _ = fmt.Fprintf(p.Out, "  Auth: %s\n", output.Faint("Build-level credentials"))
		_, _ = fmt.Fprintf(p.Out, "  Scope: %s\n", output.Faint("Build-level access"))
		renderServerInfo(p, s)

	case s.Status == "authenticated":
		_, _ = fmt.Fprintf(p.Out, "%s Logged in to %s%s\n", output.Green("✓"), output.Cyan(s.Server), suffix)
		_, _ = fmt.Fprintf(p.Out, "  User: %s (%s) · %s\n", s.User.Name, s.User.Username, tokenSourceLabel(s.TokenSource))
		renderTokenExpiry(p, s.TokenExpiry)
		renderServerInfo(p, s)

	case s.Status == "error" && s.AuthMethod == "":
		_, _ = fmt.Fprintf(p.Out, "%s %s%s\n", output.Red("✗"), s.Server, suffix)
		renderCredentialsDiagnostic(p, s)

	case s.Status == "error":
		_, _ = fmt.Fprintf(p.Out, "%s Server: %s%s\n", output.Red("✗"), s.Server, suffix)
		_, _ = fmt.Fprintf(p.Out, "  %s\n", s.Error)
	}
}

func renderServerInfo(p *output.Printer, s authStatus) {
	if s.ServerInfo == nil {
		return
	}
	_, _ = fmt.Fprintf(p.Out, "  Server: TeamCity %d.%d (build %s)\n",
		s.ServerInfo.VersionMajor, s.ServerInfo.VersionMinor, s.ServerInfo.BuildNumber)
	if s.versionCheckErr != "" {
		_, _ = fmt.Fprintf(p.Out, "  %s %s\n", output.Yellow("!"), s.versionCheckErr)
	} else {
		_, _ = fmt.Fprintf(p.Out, "  %s API compatible\n", output.Green("✓"))
	}
}

func renderTokenExpiry(p *output.Printer, expiry string) {
	if expiry == "" {
		return
	}
	t, err := time.Parse(time.RFC3339, expiry)
	if err != nil {
		return
	}
	remaining := time.Until(t)
	switch {
	case remaining <= 0:
		_, _ = fmt.Fprintf(p.Out, "  %s Token expired on %s\n", output.Red("✗"), t.Local().Format("Jan 2, 2006"))
		_, _ = fmt.Fprintf(p.Out, "  Run %s to re-authenticate\n", output.Cyan("teamcity auth login"))
	case remaining <= 3*24*time.Hour:
		_, _ = fmt.Fprintf(p.Out, "  %s Token expires %s (on %s)\n",
			output.Yellow("!"), output.Yellow(humanize.Time(t)), t.Local().Format("Jan 2, 2006"))
	default:
		_, _ = fmt.Fprintf(p.Out, "  Token expires: %s\n", t.Local().Format("Jan 2, 2006"))
	}
}

func renderCredentialsDiagnostic(p *output.Printer, s authStatus) {
	if s.configUser != "" {
		if s.keyringErr != nil {
			_, _ = fmt.Fprintf(p.Out, "  Token is in the system keyring but could not be retrieved: %v\n", s.keyringErr)
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
		output.Cyan("teamcity auth login --server "+s.Server+" --insecure-storage"))
	if cmdutil.ProbeGuestAccess(s.Server) {
		_, _ = fmt.Fprintf(p.Out, "    • Or set %s for read-only guest access\n", output.Cyan("TEAMCITY_GUEST=1"))
	}
}

func renderDSLHint(p *output.Printer, results []authStatus) {
	dslURL := config.DetectServerFromDSL()
	if dslURL == "" {
		return
	}
	for _, s := range results {
		if s.Server == dslURL {
			return
		}
	}
	_, _ = fmt.Fprintln(p.Out)
	_, _ = fmt.Fprintf(p.Out, "%s Commands in this directory target %s (from DSL settings)\n",
		output.Yellow("!"), output.Cyan(dslURL))
	loginCmd := output.Cyan("teamcity auth login --server " + dslURL)
	if cmdutil.ProbeGuestAccess(dslURL) {
		_, _ = fmt.Fprintf(p.Out, "  Run %s, or set %s for guest access\n", loginCmd, output.Cyan("TEAMCITY_GUEST=1"))
	} else {
		_, _ = fmt.Fprintf(p.Out, "  Run %s to authenticate\n", loginCmd)
	}
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
