package cmd

import (
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

	cmd.PersistentFlags().BoolVar(&f.NoColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().BoolVarP(&f.Quiet, "quiet", "q", false, "Suppress non-essential output")
	cmd.PersistentFlags().BoolVarP(&f.Verbose, "verbose", "V", false, "Show detailed output including debug info")
	cmd.PersistentFlags().BoolVar(&f.Verbose, "debug", false, "Alias for --verbose")
	cmd.PersistentFlags().BoolVar(&f.NoInput, "no-input", false, "Disable interactive prompts")

	cmd.MarkFlagsMutuallyExclusive("quiet", "verbose", "debug")

	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		f.InitOutput()
		if jsonFlag := cmd.Flags().Lookup("json"); jsonFlag != nil && jsonFlag.Changed {
			if jsonFlag.Value.Type() != "bool" || jsonFlag.Value.String() != "false" {
				f.JSONOutput = true
			}
		}
		if cmd.Name() != "update" && f.UpdateNotice == nil {
			f.UpdateNotice = update.CheckInBackground(f.Printer.ErrOut, f.Quiet)
		}
	}

	cmd.AddCommand(auth.NewCmd(f))
	cmd.AddCommand(project.NewCmd(f))
	cmd.AddCommand(job.NewCmd(f))
	cmd.AddCommand(run.NewCmd(f))
	cmd.AddCommand(queue.NewCmd(f))
	cmd.AddCommand(agent.NewCmd(f))
	cmd.AddCommand(pool.NewCmd(f))
	cmd.AddCommand(pipeline.NewCmd(f))
	cmd.AddCommand(apicmd.NewCmd(f))
	cmd.AddCommand(skill.NewCmd(f))
	cmd.AddCommand(configcmd.NewCmd(f))
	cmd.AddCommand(alias.NewCmd(f))
	cmd.AddCommand(updatecmd.NewCmd(f))

	return cmd
}

func Execute() error {
	f := cmdutil.NewFactory()
	rootCmd := buildRootCmd(f)

	RegisterAliases(rootCmd, f)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	executedCmd, err := rootCmd.ExecuteC()
	if f.UpdateNotice != nil {
		f.UpdateNotice()
	}
	if !f.JSONOutput && executedCmd != nil {
		if jsonFlag := executedCmd.Flags().Lookup("json"); jsonFlag != nil && jsonFlag.Changed {
			if jsonFlag.Value.Type() != "bool" || jsonFlag.Value.String() != "false" {
				f.JSONOutput = true
			}
		}
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
	var ue api.UserError
	return errors.As(err, &ue) && ue.Category() == cat
}

// RegisterAliases forwards to alias.RegisterAliases.
func RegisterAliases(rootCmd *cobra.Command, f *cmdutil.Factory) {
	alias.RegisterAliases(rootCmd, f)
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
