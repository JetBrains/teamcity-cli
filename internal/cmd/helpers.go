package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// viewOptions is shared by view commands that support JSON and web output.
type viewOptions struct {
	json bool
	web  bool
}

// addViewFlags adds --json and --web flags to a command.
func addViewFlags(cmd *cobra.Command, opts *viewOptions) {
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")
}

// parseID converts a string argument to an integer ID.
// Used for parsing agent and pool IDs from command line arguments.
func parseID(s string, entity string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s ID: %s (must be a number)", entity, s)
	}
	return id, nil
}
