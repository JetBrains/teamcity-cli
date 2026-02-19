package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/buildkite/shellwords"
	"github.com/spf13/cobra"
)

const maxAliasDepth = 10

// aliasDepth tracks recursive alias expansion. Safe without synchronization
// because cobra executes a single command tree per Execute() call.
var aliasDepth int

func RegisterAliases(rootCmd *cobra.Command) {
	for name, expansion := range config.GetAllAliases() {
		if isBuiltinCommand(rootCmd, name) {
			output.Debug("skipping alias %q: conflicts with built-in command", name)
			continue
		}
		if exp, shell := config.ParseExpansion(expansion); shell {
			rootCmd.AddCommand(newShellAliasCmd(name, exp))
		} else {
			rootCmd.AddCommand(newExpansionAliasCmd(name, exp))
		}
	}
}

func isBuiltinCommand(rootCmd *cobra.Command, name string) bool {
	for _, c := range rootCmd.Commands() {
		if c.Name() == name || c.HasAlias(name) {
			return true
		}
	}
	return false
}

func newExpansionAliasCmd(name, expansion string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              fmt.Sprintf("Alias for %q", expansion),
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if hasHelpFlag(args) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Alias for %q\n", expansion)
				return nil
			}

			if aliasDepth >= maxAliasDepth {
				return fmt.Errorf("alias expansion depth limit exceeded (possible infinite loop)")
			}
			aliasDepth++
			defer func() { aliasDepth-- }()

			expanded, err := expandArgs(expansion, args)
			if err != nil {
				return err
			}
			root := cmd.Root()
			root.SetArgs(expanded)
			return root.Execute()
		},
	}
}

func newShellAliasCmd(name, expansion string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              fmt.Sprintf("Shell alias for %q", expansion),
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if hasHelpFlag(args) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Shell alias for %q\n", expansion)
				return nil
			}
			expanded := expandShellArgs(expansion, args)
			//nolint:gosec // shell aliases are user-defined, intentional shell execution
			c := exec.Command("sh", "-c", expanded)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
}

// expandArgs substitutes $1..$N placeholders in expansion with args.
// Replacement runs in reverse order so $10 is replaced before $1.
// Unmatched args are appended to the result.
func expandArgs(expansion string, args []string) ([]string, error) {
	var extraArgs []string
	for i := len(args) - 1; i >= 0; i-- {
		placeholder := fmt.Sprintf("$%d", i+1)
		if strings.Contains(expansion, placeholder) {
			expansion = strings.ReplaceAll(expansion, placeholder, args[i])
		} else {
			extraArgs = append(extraArgs, args[i])
		}
	}
	slices.Reverse(extraArgs)
	parts, err := shellwords.Split(expansion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse alias expansion: %w", err)
	}
	return append(parts, extraArgs...), nil
}

// expandShellArgs substitutes $1..$N placeholders in a shell expression.
// Replacement runs in reverse order so $10 is replaced before $1.
// Arguments are shell-quoted to prevent injection.
func expandShellArgs(expansion string, args []string) string {
	for i := len(args) - 1; i >= 0; i-- {
		placeholder := fmt.Sprintf("$%d", i+1)
		expansion = strings.ReplaceAll(expansion, placeholder, shellwords.QuotePosix(args[i]))
	}
	return expansion
}

func hasHelpFlag(args []string) bool {
	return slices.Contains(args, "--help") || slices.Contains(args, "-h")
}
