package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestCommandPathForAnalytics_AliasExpansion locks down alias-expansion path resolution: the recorded command must be the real subcommand chain, with positional placeholders, literal endpoints, and trailing flags excluded.
func TestCommandPathForAnalytics_AliasExpansion(t *testing.T) {
	mkRoot := func() *cobra.Command {
		root := &cobra.Command{Use: "teamcity"}
		run := &cobra.Command{Use: "run"}
		run.AddCommand(&cobra.Command{Use: "list"}, &cobra.Command{Use: "view"}, &cobra.Command{Use: "log"})
		root.AddCommand(run, &cobra.Command{Use: "api"})
		return root
	}
	cases := map[string]struct {
		expansion string
		want      string
	}{
		"plain expansion":            {"run list", "run.list"},
		"trailing flags":             {"run log --tail 200", "run.log"},
		"positional placeholder":     {"run view $1", "run.view"},
		"literal endpoint after api": {"api /app/rest/server", "api"},
		"unknown leading word":       {"nope --foo", "x"}, // falls back to alias name
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := mkRoot()
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
