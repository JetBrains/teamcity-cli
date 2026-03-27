package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd/agent"
	"github.com/JetBrains/teamcity-cli/internal/cmd/alias"
	apicmd "github.com/JetBrains/teamcity-cli/internal/cmd/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd/auth"
	"github.com/JetBrains/teamcity-cli/internal/cmd/job"
	"github.com/JetBrains/teamcity-cli/internal/cmd/pool"
	"github.com/JetBrains/teamcity-cli/internal/cmd/project"
	"github.com/JetBrains/teamcity-cli/internal/cmd/queue"
	"github.com/JetBrains/teamcity-cli/internal/cmd/run"
	"github.com/JetBrains/teamcity-cli/internal/cmd/skill"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

var Version = "dev"

func buildRootCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
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
			out := f.Printer.Out
			output.PrintLogo(out)
			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintln(out, "TeamCity CLI "+output.Faint("v"+Version)+" - "+output.Faint("https://jb.gg/tc/docs"))
			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintln(out, "Usage: teamcity <command> [flags]")
			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintln(out, "Common commands:")
			_, _ = fmt.Fprintln(out, "  auth login              Authenticate with TeamCity")
			_, _ = fmt.Fprintln(out, "  run list                List recent runs")
			_, _ = fmt.Fprintln(out, "  run start <job>         Trigger a new run")
			_, _ = fmt.Fprintln(out, "  run view <id>           View run details")
			_, _ = fmt.Fprintln(out, "  job list                List jobs")
			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintln(out, output.Faint("Run 'teamcity --help' for full command list, or 'teamcity <command> --help' for details"))
		},
	}

	cmd.SetVersionTemplate("teamcity version {{.Version}}\n")
	cmd.SuggestionsMinimumDistance = 2

	cmd.PersistentFlags().BoolVar(&f.NoColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().BoolVarP(&f.Quiet, "quiet", "q", false, "Suppress non-essential output")
	cmd.PersistentFlags().BoolVar(&f.Verbose, "verbose", false, "Show detailed output including debug info")
	cmd.PersistentFlags().BoolVar(&f.NoInput, "no-input", false, "Disable interactive prompts")

	cmd.MarkFlagsMutuallyExclusive("quiet", "verbose")

	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		f.InitOutput()
	}

	cmd.AddCommand(auth.NewCmd(f))
	cmd.AddCommand(project.NewCmd(f))
	cmd.AddCommand(job.NewCmd(f))
	cmd.AddCommand(run.NewCmd(f))
	cmd.AddCommand(queue.NewCmd(f))
	cmd.AddCommand(agent.NewCmd(f))
	cmd.AddCommand(pool.NewCmd(f))
	cmd.AddCommand(apicmd.NewCmd(f))
	cmd.AddCommand(skill.NewCmd(f))
	cmd.AddCommand(alias.NewCmd(f))

	return cmd
}

func Execute() error {
	f := cmdutil.NewFactory()
	rootCmd := buildRootCmd(f)

	RegisterAliases(rootCmd, f.Printer)
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err != nil && errors.Is(err, api.ErrAuthentication) {
		tryAutoReauth(f)
	}
	if err != nil {
		if _, ok := errors.AsType[*cmdutil.ExitError](err); !ok {
			_, _ = fmt.Fprintf(f.Printer.ErrOut, "Error: %v\n", enrichAPIError(err))
		}
	}
	return err
}

func tryAutoReauth(f *cmdutil.Factory) {
	if !f.IsInteractive() {
		return
	}
	expiry := config.GetTokenExpiry()
	if expiry == "" {
		return
	}
	t, err := time.Parse(time.RFC3339, expiry)
	if err != nil || time.Until(t) > 0 {
		return
	}
	_, _ = fmt.Fprintf(f.Printer.ErrOut, "\n%s Token expired. Run %s to re-authenticate.\n", output.Yellow("!"), output.Cyan("teamcity auth login"))
}

func enrichAPIError(err error) error {
	if errors.Is(err, api.ErrReadOnly) {
		return tcerrors.WithSuggestion(
			err.Error(),
			"Unset the TEAMCITY_RO environment variable to allow write operations",
		)
	}

	if errors.Is(err, api.ErrAuthentication) {
		return tcerrors.WithSuggestion(
			"Authentication failed: invalid or expired token",
			"Run 'teamcity auth login' to re-authenticate",
		)
	}

	if _, ok := errors.AsType[*api.PermissionError](err); ok {
		return tcerrors.WithSuggestion(err.Error(), "Check your TeamCity permissions or contact your administrator")
	}

	if _, ok := errors.AsType[*api.NotFoundError](err); ok {
		return tcerrors.WithSuggestion(err.Error(), notFoundHint(err.Error()))
	}

	if _, ok := errors.AsType[*api.NetworkError](err); ok {
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

// RegisterAliases forwards to alias.RegisterAliases.
func RegisterAliases(rootCmd *cobra.Command, p *output.Printer) {
	alias.RegisterAliases(rootCmd, p)
}

// RootCommand is an alias for cobra.Command for external access
type RootCommand = cobra.Command

// GetRootCmd returns a root command for doc generation and external access.
func GetRootCmd() *RootCommand {
	f := cmdutil.NewFactory()
	return buildRootCmd(f)
}

// NewRootCmd creates a fresh root command instance for testing.
// Unlike the production root, it does not set PersistentPreRun to avoid
// races on output globals when tests run in parallel.
func NewRootCmd() *RootCommand {
	f := cmdutil.NewFactory()
	cmd := buildRootCmd(f)
	cmd.PersistentPreRun = nil
	return cmd
}

// NewRootCmdWithFactory creates a fresh root command with a specific factory (for tests).
func NewRootCmdWithFactory(f *cmdutil.Factory) *RootCommand {
	cmd := buildRootCmd(f)
	cmd.PersistentPreRun = nil
	return cmd
}
