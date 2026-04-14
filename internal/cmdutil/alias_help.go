package cmdutil

import (
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// AliasAwareHelp makes help output reflect whichever alias the user typed.
func AliasAwareHelp(cmd *cobra.Command, canonical, alias string) {
	base := cmd.HelpFunc()
	replacer := buildNounReplacer(canonical, alias)

	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		target, name := detectAlias(c, cmd)
		if target == nil {
			base(c, args)
			return
		}
		swap := snapshotAndSwap(c, target, name, replacer)
		base(c, args)
		swap.restore()
	})
}

type helpSnapshot struct {
	aliased     *cobra.Command
	origUse     string
	origAliases []string
	helpCmd     *cobra.Command
	origShort   string
	origLong    string
	origExample string
	subShorts   []savedShort
}

type savedShort struct {
	cmd  *cobra.Command
	orig string
}

func (s *helpSnapshot) restore() {
	s.aliased.Use = s.origUse
	s.aliased.Aliases = s.origAliases
	s.helpCmd.Short, s.helpCmd.Long, s.helpCmd.Example = s.origShort, s.origLong, s.origExample
	for _, b := range s.subShorts {
		b.cmd.Short = b.orig
	}
}

func snapshotAndSwap(helpCmd, aliased *cobra.Command, called string, r *strings.Replacer) *helpSnapshot {
	snap := &helpSnapshot{
		aliased:     aliased,
		origUse:     aliased.Use,
		origAliases: aliased.Aliases,
		helpCmd:     helpCmd,
		origShort:   helpCmd.Short,
		origLong:    helpCmd.Long,
		origExample: helpCmd.Example,
	}

	aliased.Use = called
	swapped := make([]string, 0, len(snap.origAliases))
	for _, a := range snap.origAliases {
		if a != called {
			swapped = append(swapped, a)
		}
	}
	swapped = append(swapped, snap.origUse)
	aliased.Aliases = swapped

	if r != nil {
		helpCmd.Short = r.Replace(helpCmd.Short)
		helpCmd.Long = r.Replace(helpCmd.Long)
		helpCmd.Example = r.Replace(helpCmd.Example)
		for _, sub := range helpCmd.Commands() {
			if replaced := r.Replace(sub.Short); replaced != sub.Short {
				snap.subShorts = append(snap.subShorts, savedShort{sub, sub.Short})
				sub.Short = replaced
			}
		}
	}

	return snap
}

// detectAlias checks whether the command (or a parent) was invoked via alias.
func detectAlias(c, aliasRoot *cobra.Command) (*cobra.Command, string) {
	if ca := c.CalledAs(); ca != "" && ca != c.Name() {
		return c, ca
	}
	// Cobra only sets CalledAs on the leaf command; for parents we check os.Args.
	for _, alias := range aliasRoot.Aliases {
		if slices.Contains(os.Args[1:], alias) {
			return aliasRoot, alias
		}
	}
	return nil, ""
}

// buildNounReplacer generates old→new pairs from canonical/alias nouns.
func buildNounReplacer(canonical, alias string) *strings.Replacer {
	if canonical == "" || alias == "" {
		return nil
	}
	cp, ap := canonical+"s", alias+"s"
	return strings.NewReplacer(
		cp+" ("+ap+")", ap+" ("+cp+")",
		"a new "+canonical, "a new "+alias,
		"a "+canonical, "a "+alias,
		"two "+cp, "two "+ap,
		"recent "+cp, "recent "+ap,
		cp, ap,
		"teamcity "+canonical+" ", "teamcity "+alias+" ",
	)
}
