package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"

	NoColor bool
	Quiet   bool
	Verbose bool
	NoInput bool
)

var rootCmd = &cobra.Command{
	Use:   "teamcity",
	Short: "TeamCity CLI",
	Long: "TeamCity CLI v" + Version + `

A command-line interface for interacting with TeamCity CI/CD server.

teamcity provides a complete experience for managing
TeamCity runs, jobs, projects and more from the command line.

Documentation:  https://jb.gg/tc/docs
Report issues:  https://jb.gg/tc/issues`,
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		output.PrintLogo()
		fmt.Println()
		fmt.Println("TeamCity CLI " + output.Faint("v"+Version) + " - " + output.Faint("https://jb.gg/tc/docs"))
		fmt.Println()
		fmt.Println("Usage: teamcity <command> [flags]")
		fmt.Println()
		fmt.Println("Common commands:")
		fmt.Println("  auth login              Authenticate with TeamCity")
		fmt.Println("  run list                List recent runs")
		fmt.Println("  run start <job>         Trigger a new run")
		fmt.Println("  run view <id>           View run details")
		fmt.Println("  job list                List jobs")
		fmt.Println()
		fmt.Println(output.Faint("Run 'teamcity --help' for full command list, or 'teamcity <command> --help' for details"))
	},
}

func init() {
	rootCmd.SetVersionTemplate("teamcity version {{.Version}}\n")
	rootCmd.SuggestionsMinimumDistance = 2

	rootCmd.PersistentFlags().BoolVar(&NoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&Quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "Show detailed output including debug info")
	rootCmd.PersistentFlags().BoolVar(&NoInput, "no-input", false, "Disable interactive prompts")

	rootCmd.MarkFlagsMutuallyExclusive("quiet", "verbose")

	cobra.OnInitialize(initColorSettings)

	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newProjectCmd())
	rootCmd.AddCommand(newJobCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newQueueCmd())
	rootCmd.AddCommand(newAgentCmd())
	rootCmd.AddCommand(newPoolCmd())
	rootCmd.AddCommand(newAPICmd())
	rootCmd.AddCommand(newSkillCmd())
	rootCmd.AddCommand(newAliasCmd())
}

func initColorSettings() {
	output.Quiet = Quiet
	output.Verbose = Verbose

	if os.Getenv("NO_COLOR") != "" ||
		os.Getenv("TERM") == "dumb" ||
		NoColor ||
		!isatty.IsTerminal(os.Stdout.Fd()) {
		color.NoColor = true
	}
}

func Execute() error {
	RegisterAliases(rootCmd)
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err != nil {
		var exitErr *ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", enrichAPIError(err))
		}
	}
	return err
}

// tryAutoReauth attempts PKCE re-authentication when a token has expired.
// Returns true if re-auth succeeded and the user should re-run their command.
func tryAutoReauth() bool {
	if !output.IsStdinTerminal() || NoInput {
		return false
	}
	serverURL := config.GetServerURL()
	if serverURL == "" {
		return false
	}
	expiry := config.GetTokenExpiry()
	if expiry == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, expiry)
	if err != nil || time.Until(t) > 0 {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	enabled, _ := api.IsPkceEnabled(ctx, serverURL)
	if !enabled {
		return false
	}

	output.Warn("Token expired. Re-authenticating via browser...")
	scopes := api.DefaultScopes()
	if serverScopes, err := api.FetchPkceScopes(ctx, serverURL); err == nil {
		scopes = serverScopes
	}
	tokenResp, err := runPkceLogin(serverURL, scopes)
	if err != nil {
		output.Warn("Auto re-authentication failed: %v", err)
		return false
	}

	user := config.GetCurrentUser()
	if _, err := config.SetServerWithKeyring(serverURL, tokenResp.AccessToken, user, tokenResp.ValidUntil, false); err != nil {
		return false
	}

	output.Success("Token refreshed successfully")
	return true
}

// enrichAPIError converts typed API errors into UserErrors with CLI-specific hints.
func enrichAPIError(err error) error {
	if errors.Is(err, api.ErrAuthentication) {
		if tryAutoReauth() {
			return tcerrors.WithSuggestion(
				"Token was refreshed automatically",
				"Please re-run your command",
			)
		}
		return tcerrors.WithSuggestion(
			"Authentication failed: invalid or expired token",
			"Run 'teamcity auth login' to re-authenticate",
		)
	}

	var permErr *api.PermissionError
	if errors.As(err, &permErr) {
		return tcerrors.WithSuggestion(err.Error(), "Check your TeamCity permissions or contact your administrator")
	}

	var notFoundErr *api.NotFoundError
	if errors.As(err, &notFoundErr) {
		return tcerrors.WithSuggestion(err.Error(), notFoundHint(err.Error()))
	}

	var netErr *api.NetworkError
	if errors.As(err, &netErr) {
		return tcerrors.WithSuggestion(err.Error(), "Check your network connection and verify the server URL")
	}

	return err
}

func notFoundHint(message string) string {
	msg := strings.ToLower(message)
	switch {
	case strings.Contains(msg, "agent pool"), strings.Contains(msg, "pool"):
		return "Use 'teamcity pool list' to see available pools"
	case strings.Contains(msg, "agent"):
		return "Use 'teamcity agent list' to see available agents"
	case strings.Contains(msg, "project"):
		return "Use 'teamcity project list' to see available projects"
	case strings.Contains(msg, "build type"), strings.Contains(msg, "job"):
		return "Use 'teamcity job list' to see available jobs"
	default:
		return "Use 'teamcity job list' or 'teamcity run list' to see available resources"
	}
}

// subcommandRequired is a RunE function for parent commands that require a subcommand.
// It returns an error when no valid subcommand is provided.
func subcommandRequired(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("requires a subcommand\n\nRun '%s --help' for available commands", cmd.CommandPath())
}

// RootCommand is an alias for cobra.Command for external access
type RootCommand = cobra.Command

// GetRootCmd returns the root command for testing
func GetRootCmd() *RootCommand {
	return rootCmd
}

// NewRootCmd creates a fresh root command instance for testing.
// This ensures tests don't share flag state from previous test runs.
// Callers must call RegisterAliases explicitly if alias expansion is needed.
func NewRootCmd() *RootCommand {
	cmd := &cobra.Command{
		Use:     "teamcity",
		Short:   "TeamCity CLI",
		Version: Version,
	}

	cmd.PersistentFlags().BoolVar(&NoColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().BoolVarP(&Quiet, "quiet", "q", false, "Suppress non-essential output")
	cmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "Show detailed output including debug info")
	cmd.PersistentFlags().BoolVar(&NoInput, "no-input", false, "Disable interactive prompts")

	cmd.AddCommand(newAuthCmd())
	cmd.AddCommand(newProjectCmd())
	cmd.AddCommand(newJobCmd())
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newQueueCmd())
	cmd.AddCommand(newAgentCmd())
	cmd.AddCommand(newPoolCmd())
	cmd.AddCommand(newAPICmd())
	cmd.AddCommand(newSkillCmd())
	cmd.AddCommand(newAliasCmd())

	return cmd
}
