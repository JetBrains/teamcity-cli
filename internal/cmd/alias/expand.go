package alias

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/buildkite/shellwords"
	"github.com/spf13/cobra"
)

const maxAliasDepth = 10

var aliasDepth int

func RegisterAliases(rootCmd *cobra.Command, f *cmdutil.Factory) {
	for name, expansion := range config.GetAllAliases() {
		if isBuiltinCommand(rootCmd, name) {
			f.Printer.Debug("skipping alias %q: conflicts with built-in command", name)
			continue
		}
		if exp, shell := config.ParseExpansion(expansion); shell {
			rootCmd.AddCommand(newShellAliasCmd(f, name, exp))
		} else {
			rootCmd.AddCommand(newExpansionAliasCmd(f.Printer, name, exp))
		}
	}
}

func isBuiltinCommand(rootCmd *cobra.Command, name string) bool {
	for _, c := range rootCmd.Commands() {
		if c.Annotations["is_alias"] == "true" {
			continue
		}
		if c.Name() == name || c.HasAlias(name) {
			return true
		}
	}
	return false
}

func newExpansionAliasCmd(p *output.Printer, name, expansion string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              fmt.Sprintf("Alias for %q", expansion),
		Annotations:        map[string]string{"is_alias": "true"},
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if hasHelpFlag(args) {
				_, _ = fmt.Fprintf(p.Out, "Alias for %q\n", expansion)
				return nil
			}

			if aliasDepth >= maxAliasDepth {
				return errors.New("alias expansion depth limit exceeded (possible infinite loop)")
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

func newShellAliasCmd(f *cmdutil.Factory, name, expansion string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              fmt.Sprintf("Shell alias for %q", expansion),
		Annotations:        map[string]string{"is_alias": "true"},
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if hasHelpFlag(args) {
				_, _ = fmt.Fprintf(f.Printer.Out, "Shell alias for %q\n", expansion)
				return nil
			}
			expanded := expandShellArgs(expansion, args)
			//nolint:gosec // shell aliases are user-defined, intentional shell execution
			c := exec.Command("sh", "-c", expanded)
			c.Stdin = f.IOStreams.In
			c.Stdout = f.Printer.Out
			c.Stderr = f.Printer.ErrOut
			return c.Run()
		},
	}
}

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
