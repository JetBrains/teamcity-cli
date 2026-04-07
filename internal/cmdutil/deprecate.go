package cmdutil

import (
	"fmt"

	"github.com/spf13/cobra"
)

// DeprecateFlag marks a flag as deprecated with a consistent message format.
func DeprecateFlag(cmd *cobra.Command, old, new, removeVersion string) {
	msg := fmt.Sprintf("use --%s instead (will be removed in %s)", new, removeVersion)
	if flag := cmd.LocalFlags().Lookup(old); flag != nil {
		flag.Deprecated = msg
		flag.Hidden = true
		return
	}

	panic(fmt.Sprintf("DeprecateFlag: flag %q not found on command %q", old, cmd.Name()))
}

// DeprecateCommand marks a command as deprecated with a consistent message format.
func DeprecateCommand(cmd *cobra.Command, new, removeVersion string) {
	cmd.Deprecated = fmt.Sprintf("use %q instead (will be removed in %s)", new, removeVersion)
}
