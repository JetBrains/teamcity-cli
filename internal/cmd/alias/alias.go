package alias

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage command aliases",
		Long: `Create, list, and delete command shortcuts.

Aliases expand to full teamcity commands, so you can type 'tc rl'
instead of 'tc run list'. Aliases support positional placeholders
($1, $2, ...) and shell-style expansions via the --shell flag.`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newAliasSetCmd(f))
	cmd.AddCommand(newAliasListCmd(f))
	cmd.AddCommand(newAliasDeleteCmd(f))

	return cmd
}

func newAliasSetCmd(f *cmdutil.Factory) *cobra.Command {
	var shell bool

	cmd := &cobra.Command{
		Use:   "set <name> <expansion>",
		Short: "Create a command alias",
		Long: `Create a shortcut that expands into a full teamcity command.

Use $1, $2, ... for positional arguments. Extra arguments are appended.
Use --shell for aliases that need pipes, redirection, or other shell features.`,
		Example: `  # Quick shortcuts
  teamcity alias set rl  'run list'
  teamcity alias set rw  'run view $1 --web'

  # Filtered views
  teamcity alias set mine    'run list --user=@me'
  teamcity alias set fails   'run list --status=failure --since=24h'
  teamcity alias set running 'run list --status=running'

  # Trigger-and-watch workflows
  teamcity alias set go    'run start $1 --watch'
  teamcity alias set hotfix 'run start $1 --top --clean --watch'

  # Shell aliases for pipes and external tools
  teamcity alias set watchnotify '!teamcity run watch $1 && notify-send "Build $1 done"'
  teamcity alias set faillog '!teamcity run list --status=failure --json | jq ".[].id"'`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, expansion := args[0], args[1]

			if isBuiltinCommand(cmd.Root(), name) {
				return fmt.Errorf("%q is a built-in command and cannot be used as an alias", name)
			}

			if shell && !strings.HasPrefix(expansion, "!") {
				expansion = "!" + expansion
			}

			_, existed := config.GetAlias(name)
			if err := config.AddAlias(name, expansion); err != nil {
				return err
			}

			if existed {
				f.Printer.Success("Changed alias %q", name)
			} else {
				f.Printer.Success("Added alias %q", name)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&shell, "shell", false, "Evaluate expansion as a shell expression via sh")

	return cmd
}

type aliasEntry struct {
	Name      string `json:"name"`
	Expansion string `json:"expansion"`
	Shell     bool   `json:"shell"`
	Type      string `json:"type"`
}

func newAliasListCmd(f *cmdutil.Factory) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List configured aliases",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Example: `  teamcity alias list
  teamcity alias list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			builtins := collectBuiltinAliases(cmd.Root())
			aliases := config.GetAllAliases()

			if len(builtins) == 0 && len(aliases) == 0 {
				_, _ = fmt.Fprintln(f.Printer.Out, "No aliases configured. Use \"teamcity alias set\" to create one.")
				return nil
			}

			builtinNames := slices.Sorted(maps.Keys(builtins))
			userNames := slices.Sorted(maps.Keys(aliases))

			if jsonOutput {
				entries := make([]aliasEntry, 0, len(builtins)+len(aliases))
				for _, name := range builtinNames {
					entries = append(entries, aliasEntry{
						Name:      name,
						Expansion: builtins[name],
						Type:      "built-in",
					})
				}
				for _, name := range userNames {
					displayExp, isShell := config.ParseExpansion(aliases[name])
					kind := "expansion"
					if isShell {
						kind = "shell"
					}
					entries = append(entries, aliasEntry{
						Name:      name,
						Expansion: displayExp,
						Shell:     isShell,
						Type:      kind,
					})
				}
				return f.Printer.PrintJSON(entries)
			}

			headers := []string{"NAME", "EXPANSION", "TYPE"}
			var rows [][]string
			for _, name := range builtinNames {
				rows = append(rows, []string{name, builtins[name], "built-in"})
			}
			for _, name := range userNames {
				displayExp, isShell := config.ParseExpansion(aliases[name])
				aliasType := "expansion"
				if isShell {
					aliasType = "shell"
				}
				rows = append(rows, []string{name, displayExp, aliasType})
			}
			f.Printer.PrintTable(headers, rows)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func collectBuiltinAliases(rootCmd *cobra.Command) map[string]string {
	result := map[string]string{}
	for _, c := range rootCmd.Commands() {
		for _, a := range c.Aliases {
			result[a] = c.Name()
		}
	}
	return result
}

func newAliasDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:               "delete <name>",
		Short:             "Delete an alias",
		Aliases:           []string{"rm"},
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.AliasNames(),
		Example:           `  teamcity alias delete rl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := config.DeleteAlias(name); err != nil {
				return err
			}
			f.Printer.Success("Deleted alias %q", name)
			return nil
		},
	}
}
