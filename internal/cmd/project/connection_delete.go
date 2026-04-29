package project

import (
	"cmp"
	"fmt"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type connectionDeleteOptions struct {
	project string
	force   bool
}

func newConnectionDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &connectionDeleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a project connection",
		Long: `Delete an OAuth/connection feature by id.

The id is the one shown in the first column of 'connection list'.`,
		Example: `  teamcity project connection delete PROJECT_EXT_42 -p Backend
  teamcity project connection delete PROJECT_EXT_42 -p Backend --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnectionDelete(f, opts, args[0])
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Skip confirmation")

	return cmd
}

func runConnectionDelete(f *cmdutil.Factory, opts *connectionDeleteOptions, id string) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	projectID := cmp.Or(opts.project, "_Root")

	if !opts.force && f.IsInteractive() {
		confirm := false
		if err := cmdutil.Confirm(fmt.Sprintf("Delete connection %s from %s?", id, projectID), &confirm); err != nil {
			return err
		}
		if !confirm {
			f.Printer.Info("Canceled")
			return nil
		}
	}

	if err := client.DeleteProjectFeature(projectID, id); err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}

	f.Printer.Success("Deleted connection %s from project %s", id, projectID)
	return nil
}
