package cmd_test

import (
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/spf13/cobra"
)

// TestCommandEnumCoversCobraTree fails if any visible cobra leaf has no matching analytics command enum value, so missing entries can't silently collapse to "other" again.
func TestCommandEnumCoversCobraTree(t *testing.T) {
	root := cmd.NewCommand(nil)
	var leaves []string
	var walk func(c *cobra.Command, path []string)
	walk = func(c *cobra.Command, path []string) {
		if skipForAnalytics(c) {
			return
		}
		visible := 0
		for _, child := range c.Commands() {
			if skipForAnalytics(child) {
				continue
			}
			visible++
			walk(child, append(path, child.Name()))
		}
		if visible == 0 && len(path) > 0 {
			leaves = append(leaves, strings.Join(path, "."))
		}
	}
	walk(root, nil)

	if len(leaves) == 0 {
		t.Fatal("walked the cobra tree and found no leaves; the test is broken")
	}
	for _, p := range leaves {
		if got := analytics.NormalizeCommand(p); got != p {
			t.Errorf("cobra leaf %q normalizes to %q — add it to analytics.allCommands", p, got)
		}
	}
}

func skipForAnalytics(c *cobra.Command) bool {
	if c.Hidden {
		return true
	}
	switch c.Name() {
	case "help", "completion", "__complete", "__completeNoDesc":
		return true
	}
	return false
}
