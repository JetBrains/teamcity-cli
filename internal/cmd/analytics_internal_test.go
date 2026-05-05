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

	t.Run("chained aliases resolve to deepest real command", func(t *testing.T) {
		root := mkRoot()
		b := &cobra.Command{Use: "b", Annotations: map[string]string{"is_alias": "true", "alias_expansion": "run list"}}
		a := &cobra.Command{Use: "a", Annotations: map[string]string{"is_alias": "true", "alias_expansion": "b"}}
		root.AddCommand(a, b)
		if got := commandPathForAnalytics(a); got != "run.list" {
			t.Errorf("chained alias a → b → run list resolved to %q, want run.list", got)
		}
	})

	t.Run("cyclic aliases terminate without hanging", func(t *testing.T) {
		root := mkRoot()
		a := &cobra.Command{Use: "a", Annotations: map[string]string{"is_alias": "true", "alias_expansion": "b"}}
		b := &cobra.Command{Use: "b", Annotations: map[string]string{"is_alias": "true", "alias_expansion": "a"}}
		root.AddCommand(a, b)
		// Fallback is whichever alias the cycle terminated on; the contract under test is just "doesn't loop forever".
		if got := commandPathForAnalytics(a); got != "a" && got != "b" {
			t.Errorf("cyclic aliases resolved to unexpected %q, want a or b (alias name fallback)", got)
		}
	})
}
