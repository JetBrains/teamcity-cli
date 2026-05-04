package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestCommandPathForAnalytics_AliasExpansion locks down the fix for expansion aliases collapsing to "other"; it asserts the alias_expansion annotation drives the recorded command and that trailing flags are dropped from the path.
func TestCommandPathForAnalytics_AliasExpansion(t *testing.T) {
	cases := map[string]struct {
		expansion string
		want      string
	}{
		"plain expansion":            {"run list", "run.list"},
		"expansion with trailing flags": {"run log --tail 200", "run.log"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := &cobra.Command{Use: "teamcity"}
			alias := &cobra.Command{
				Use:         "x",
				Annotations: map[string]string{"is_alias": "true", "alias_expansion": tc.expansion},
			}
			root.AddCommand(alias)
			if got := commandPathForAnalytics(alias); got != tc.want {
				t.Errorf("alias %q → %q, want %q", tc.expansion, got, tc.want)
			}
		})
	}
}
