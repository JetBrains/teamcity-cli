package cmdutil

import "github.com/spf13/cobra"

// MarkExperimental labels a command as experimental.
//
// It does three things in one call:
//  1. Sets an "experimental" annotation (for programmatic checks).
//  2. Prepends [experimental] to Short (visible in parent help).
//  3. Wraps RunE to print a stderr notice on every invocation
//     (suppressed by --quiet).
//
// To graduate a command, remove the MarkExperimental call — no flag renames,
// no command moves, no breaking changes for early adopters.
func MarkExperimental(f *Factory, cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["experimental"] = "true"
	cmd.Short = "[experimental] " + cmd.Short

	inner := cmd.RunE
	if inner == nil {
		return
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		f.Printer.Warn("command %q is experimental and may change or be removed without notice", cmd.CommandPath())
		return inner(cmd, args)
	}
}

// IsExperimental reports whether the command (or any of its parents) is marked experimental.
func IsExperimental(cmd *cobra.Command) bool {
	if cmd.Annotations["experimental"] == "true" {
		return true
	}
	found := false
	cmd.VisitParents(func(p *cobra.Command) {
		if p.Annotations["experimental"] == "true" {
			found = true
		}
	})
	return found
}
