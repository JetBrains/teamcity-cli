package cmdutil

import "github.com/spf13/cobra"

// MarkExperimental tags a command as experimental and warns on each invocation.
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
		f.Printer.Warn("%q is experimental: flags, output, and JSON schema may change or the command may be removed without notice. It is intentionally undocumented — do not rely on it in scripts.", cmd.CommandPath())
		return inner(cmd, args)
	}
}
