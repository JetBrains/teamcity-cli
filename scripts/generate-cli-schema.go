//go:build ignore

// Dumps the CLI command tree as JSON for the skill evals: every command path
// mapped to its valid flags. Generated fresh per eval run so the allowlist in
// evals/checks.py can never rot out of sync with the binary.
//
// Usage: go run scripts/generate-cli-schema.go > evals/cli_schema.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func flagsOf(c *cobra.Command) []string {
	c.InitDefaultHelpFlag() // cobra adds --help lazily at Execute time
	var flags []string
	collect := func(f *pflag.Flag) {
		flags = append(flags, "--"+f.Name)
		if f.Shorthand != "" {
			flags = append(flags, "-"+f.Shorthand)
		}
	}
	c.Flags().VisitAll(collect)
	c.InheritedFlags().VisitAll(collect)
	return flags
}

func walk(c *cobra.Command, parentPath string, out map[string][]string) {
	names := append([]string{c.Name()}, c.Aliases...)
	for _, name := range names {
		path := strings.TrimSpace(parentPath + " " + name)
		out[path] = flagsOf(c)
		if name != c.Name() {
			continue // don't recurse under alias paths; children use the canonical parent
		}
		for _, child := range c.Commands() {
			walk(child, path, out)
		}
	}
}

func main() {
	root := cmd.NewCommand(nil)
	root.InitDefaultHelpCmd()     // `teamcity help <cmd>` is valid
	root.InitDefaultVersionFlag() // `teamcity --version/-v` is valid
	schema := map[string][]string{"": flagsOf(root)}
	for _, child := range root.Commands() {
		walk(child, "", schema)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{"commands": schema}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
