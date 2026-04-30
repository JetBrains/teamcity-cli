package cmdutil

import (
	"fmt"

	"github.com/spf13/cobra"
)

// SubcommandRequired is a RunE function for parent commands that require a subcommand.
func SubcommandRequired(cmd *cobra.Command, _ []string) error {
	return fmt.Errorf("requires a subcommand\n\nRun '%s --help' for available commands", cmd.CommandPath())
}
