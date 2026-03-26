package cmdutil

import (
	"fmt"

	"github.com/spf13/cobra"
)

// SubcommandRequired is a RunE function for parent commands that require a subcommand.
// It returns an error when no valid subcommand is provided.
func SubcommandRequired(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("requires a subcommand\n\nRun '%s --help' for available commands", cmd.CommandPath())
}
