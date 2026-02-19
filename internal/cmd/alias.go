package cmd

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAliasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage command aliases",
		Long:  "Create, list, and delete command shortcuts.",
		Args:  cobra.NoArgs,
		RunE:  subcommandRequired,
	}

	cmd.AddCommand(newAliasSetCmd())
	cmd.AddCommand(newAliasListCmd())
	cmd.AddCommand(newAliasDeleteCmd())

	return cmd
}

func newAliasSetCmd() *cobra.Command {
	var shell bool

	cmd := &cobra.Command{
		Use:   "set <name> <expansion>",
		Short: "Create a command alias",
		Long: `Create a shortcut that expands into a full tc command.

Use $1, $2, ... for positional arguments. Extra arguments are appended.
Use --shell for aliases that need pipes, redirection, or other shell features.`,
		Example: `  # Quick shortcuts
  tc alias set rl  'run list'
  tc alias set rw  'run view $1 --web'

  # Filtered views
  tc alias set mine    'run list --user=@me'
  tc alias set fails   'run list --status=failure --since=24h'
  tc alias set running 'run list --status=running'

  # Trigger-and-watch workflows
  tc alias set go    'run start $1 --watch'
  tc alias set hotfix 'run start $1 --top --clean --watch'

  # Shell aliases for pipes and external tools
  tc alias set --shell watchnotify 'tc run watch $1 && notify-send "Build $1 done"'
  tc alias set --shell faillog 'tc run list --status=failure --json | jq ".[].id"'`,
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
				output.Success("Changed alias %q", name)
			} else {
				output.Success("Added alias %q", name)
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
}

func newAliasListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured aliases",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			aliases := config.GetAllAliases()

			if len(aliases) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No aliases configured. Use \"tc alias set\" to create one.")
				return nil
			}

			names := slices.Sorted(maps.Keys(aliases))

			if jsonOutput {
				entries := make([]aliasEntry, 0, len(aliases))
				for _, name := range names {
					exp := aliases[name]
					isShell := strings.HasPrefix(exp, "!")
					displayExp := exp
					if isShell {
						displayExp = exp[1:]
					}
					entries = append(entries, aliasEntry{
						Name:      name,
						Expansion: displayExp,
						Shell:     isShell,
					})
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
			}

			headers := []string{"NAME", "EXPANSION", "TYPE"}
			var rows [][]string
			for _, name := range names {
				exp := aliases[name]
				aliasType := "expansion"
				displayExp := exp
				if strings.HasPrefix(exp, "!") {
					aliasType = "shell"
					displayExp = exp[1:]
				}
				rows = append(rows, []string{name, displayExp, aliasType})
			}
			output.PrintTable(headers, rows)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func newAliasDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := config.DeleteAlias(name); err != nil {
				return err
			}
			output.Success("Deleted alias %q", name)
			return nil
		},
	}
}
