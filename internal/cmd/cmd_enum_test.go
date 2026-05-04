package cmd_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/spf13/cobra"
)

// TestCommandEnumCoversCobraTree walks the cobra tree and fails if any visible leaf normalizes to "other"; without it, fixes like agent.term silently regress.
func TestCommandEnumCoversCobraTree(t *testing.T) {
	skip := func(c *cobra.Command) bool {
		return c.Hidden || slices.Contains([]string{"help", "completion", "__complete", "__completeNoDesc"}, c.Name())
	}
	var leaves []string
	var walk func(*cobra.Command, []string)
	walk = func(c *cobra.Command, path []string) {
		if skip(c) {
			return
		}
		kids := 0
		for _, child := range c.Commands() {
			if !skip(child) {
				kids++
				walk(child, append(path, child.Name()))
			}
		}
		if kids == 0 && len(path) > 0 {
			leaves = append(leaves, strings.Join(path, "."))
		}
	}
	walk(cmd.NewCommand(nil), nil)

	if len(leaves) == 0 {
		t.Fatal("no leaves found; test broken")
	}
	for _, p := range leaves {
		if got := analytics.NormalizeCommand(p); got != p {
			t.Errorf("cobra leaf %q normalizes to %q — add to analytics.allCommands", p, got)
		}
	}
}
