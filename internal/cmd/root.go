package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd/agent"
	"github.com/JetBrains/teamcity-cli/internal/cmd/alias"
	apicmd "github.com/JetBrains/teamcity-cli/internal/cmd/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd/auth"
	configcmd "github.com/JetBrains/teamcity-cli/internal/cmd/config"
	"github.com/JetBrains/teamcity-cli/internal/cmd/job"
	"github.com/JetBrains/teamcity-cli/internal/cmd/pipeline"
	"github.com/JetBrains/teamcity-cli/internal/cmd/pool"
	"github.com/JetBrains/teamcity-cli/internal/cmd/project"
	"github.com/JetBrains/teamcity-cli/internal/cmd/queue"
	"github.com/JetBrains/teamcity-cli/internal/cmd/run"
	"github.com/JetBrains/teamcity-cli/internal/cmd/skill"
	updatecmd "github.com/JetBrains/teamcity-cli/internal/cmd/update"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/update"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/spf13/cobra"
)

func buildRootCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "teamcity",
		Short: "TeamCity CLI",
		Long: "TeamCity CLI v" + version.String() + `

A command-line interface for interacting with TeamCity CI/CD server.

teamcity provides a complete experience for managing
TeamCity runs, jobs, projects and more from the command line.

Documentation:  https://jb.gg/tc/docs
Report issues:  https://jb.gg/tc/issues`,
		Version: version.String(),
		Run: func(cmd *cobra.Command, args []string) {
			out := f.Printer.Out
			output.PrintLogo(out)
			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintln(out, "TeamCity CLI "+output.Faint("v"+version.String())+" - "+output.Faint("https://jb.gg/tc/docs"))
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
			_, _ = fmt.Fprintln(out, output.Faint("Run 'teamcity -h' for full command list, or 'teamcity <command> -h' for details"))
		},
	}

	cmd.SetVersionTemplate("teamcity version {{.Version}}\n")
	cmd.SuggestionsMinimumDistance = 2

	cmd.AddGroup(
		&cobra.Group{ID: "core", Title: "CORE COMMANDS"},
		&cobra.Group{ID: "infra", Title: "INFRASTRUCTURE"},
		&cobra.Group{ID: "config", Title: "CONFIGURATION"},
		&cobra.Group{ID: "misc", Title: "ADDITIONAL COMMANDS"},
	)

	cmd.PersistentFlags().BoolVar(&f.NoColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().BoolVarP(&f.Quiet, "quiet", "q", false, "Suppress non-essential output")
	cmd.PersistentFlags().BoolVarP(&f.Verbose, "verbose", "V", false, "Show detailed output including debug info")
	cmd.PersistentFlags().BoolVar(&f.Verbose, "debug", false, "Alias for --verbose")
	cmd.PersistentFlags().BoolVar(&f.NoInput, "no-input", false, "Disable interactive prompts")

	cmd.MarkFlagsMutuallyExclusive("quiet", "verbose")
	cmd.MarkFlagsMutuallyExclusive("quiet", "debug")

	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		f.InitOutput()
		if jsonOutputEnabled(cmd) {
			f.JSONOutput = true
		}
		if cmd.Name() != "update" && f.UpdateNotice == nil {
			f.UpdateNotice = update.CheckInBackground(f.Context(), f.Printer.ErrOut, f.Quiet)
		}
	}

	addGrouped(cmd, "core", run.NewCmd(f), job.NewCmd(f), project.NewCmd(f), pipeline.NewCmd(f))
	addGrouped(cmd, "infra", queue.NewCmd(f), agent.NewCmd(f), pool.NewCmd(f))
	addGrouped(cmd, "config",
		auth.NewCmd(f),
		configcmd.NewCmd(f),
		alias.NewCmd(f),
		apicmd.NewCmd(f),
		skill.NewCmd(f),
		updatecmd.NewCmd(f),
	)

	cmd.SetHelpCommandGroupID("misc")
	cmd.SetCompletionCommandGroupID("misc")

	return cmd
}

func Execute(ctx context.Context) error {
	f := cmdutil.NewFactory()
	f.SetContext(ctx)
	rootCmd := buildRootCmd(f)
	rootCmd.SetContext(ctx)

	alias.RegisterAliases(rootCmd, f)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	executedCmd, err := rootCmd.ExecuteC()
	if f.UpdateNotice != nil {
		f.UpdateNotice()
	}
	if err != nil && ctx.Err() != nil {
		return nil
	}
	if !f.JSONOutput && executedCmd != nil && jsonOutputEnabled(executedCmd) {
		f.JSONOutput = true
	}
	if err != nil && isCategory(err, api.CatAuth) && !f.JSONOutput {
		tryAutoReauth(f)
	}
	if err != nil {
		if _, ok := errors.AsType[*cmdutil.ExitError](err); !ok {
			if f.JSONOutput {
				code, message, suggestion := output.ClassifyError(err)
				output.PrintJSONError(f.Printer.ErrOut, code, message, suggestion)
			} else {
				_, _ = fmt.Fprintf(f.Printer.ErrOut, "Error: %v\n", output.RenderError(err))
			}
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

func isCategory(err error, cat api.Category) bool {
	ue, ok := errors.AsType[api.UserError](err)
	return ok && ue.Category() == cat
}

// addGrouped registers subcommands under a shared group ID on the parent command.
func addGrouped(parent *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.GroupID = groupID
		parent.AddCommand(c)
	}
}

// jsonOutputEnabled reports whether --json was set to a JSON-emitting value.
// Handles both the bool --json shape (most commands) and the string
// --json=fields shape used by list commands (see cmdutil.AddJSONFieldsFlag).
func jsonOutputEnabled(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup("json")
	return f != nil && f.Changed && f.Value.String() != "false"
}

// NewCommand builds a root command for tests and doc generation.
// Pass nil for f to get a fresh production factory. PersistentPreRun is
// stripped so tests and doc walks don't spawn the update-check goroutine
// or race on output globals. Aliases are not registered — callers that
// need them invoke alias.RegisterAliases themselves.
func NewCommand(f *cmdutil.Factory) *cobra.Command {
	if f == nil {
		f = cmdutil.NewFactory()
	}
	cmd := buildRootCmd(f)
	cmd.PersistentPreRun = nil
	return cmd
}
