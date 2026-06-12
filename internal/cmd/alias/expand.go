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
		Use:   name,
		Short: fmt.Sprintf("Alias for %q", expansion),
		Annotations: map[string]string{
			"is_alias":        "true",
			"alias_expansion": expansion,
		},
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
			output.StopSpinner() // the subprocess writes to the raw fds, never through the spinner-clearing stopWriter
			//nolint:gosec // shell aliases are user-defined, intentional shell execution
			c := exec.Command("sh", "-c", expanded)
			// IOStreams, not Printer: Printer wraps stdout/stderr in a non-*os.File writer, which makes os/exec substitute pipes and the subprocess lose the TTY (no color/width, risk of Wait hanging on inherited pipe fds).
			c.Stdin = f.IOStreams.In
			c.Stdout = f.IOStreams.Out
			c.Stderr = f.IOStreams.ErrOut
			return c.Run()
		},
	}
}

func expandArgs(expansion string, args []string) ([]string, error) {
	tokens, err := shellwords.Split(expansion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse alias expansion: %w", err)
	}
	used := make([]bool, len(args))
	for i, tok := range tokens {
		for j := len(args) - 1; j >= 0; j-- {
			placeholder := fmt.Sprintf("$%d", j+1)
			if strings.Contains(tok, placeholder) {
				tok = strings.ReplaceAll(tok, placeholder, args[j])
				used[j] = true
			}
		}
		tokens[i] = tok
	}
	for j, u := range used {
		if !u {
			tokens = append(tokens, args[j])
		}
	}
	return tokens, nil
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
