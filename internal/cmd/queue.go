package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/api"
	"github.com/tiulpin/teamcity-cli/internal/output"
)

func newQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage build queue",
		Long:  `List and manage the TeamCity build queue.`,
	}

	cmd.AddCommand(newQueueListCmd())
	cmd.AddCommand(newQueueRemoveCmd())
	cmd.AddCommand(newQueueTopCmd())
	cmd.AddCommand(newQueueApproveCmd())

	return cmd
}

type queueListOptions struct {
	job        string
	limit      int
	jsonFields string
}

func newQueueListCmd() *cobra.Command {
	opts := &queueListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List queued runs",
		Long:  `List all runs in the TeamCity queue.`,
		Example: `  tc queue list
  tc queue list --job Sandbox_Demo
  tc queue list --json
  tc queue list --json=id,state,webUrl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueueList(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Filter by job ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 30, "Maximum number of queued runs")
	AddJSONFieldsFlag(cmd, &opts.jsonFields)

	return cmd
}

func runQueueList(cmd *cobra.Command, opts *queueListOptions) error {
	jsonResult, showHelp, err := ParseJSONFields(cmd, opts.jsonFields, &api.QueuedBuildFields)
	if err != nil {
		return err
	}
	if showHelp {
		return nil
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	queue, err := client.GetBuildQueue(api.QueueOptions{
		BuildTypeID: opts.job,
		Limit:       opts.limit,
		Fields:      jsonResult.Fields,
	})
	if err != nil {
		return err
	}

	if jsonResult.Enabled {
		return output.PrintJSON(queue)
	}

	if queue.Count == 0 {
		fmt.Println("No runs in queue")
		return nil
	}

	headers := []string{"ID", "JOB", "BRANCH", "STATE"}
	var rows [][]string

	widths := output.ColumnWidths(30, 40, 60, 40)

	for _, r := range queue.Builds {
		branch := r.BranchName
		if branch == "" {
			branch = "<default>"
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", r.ID),
			output.Truncate(r.BuildTypeID, widths[0]),
			output.Truncate(branch, widths[1]),
			r.State,
		})
	}

	output.PrintTable(headers, rows)
	return nil
}

type queueRemoveOptions struct {
	force bool
}

func newQueueRemoveCmd() *cobra.Command {
	opts := &queueRemoveOptions{}

	cmd := &cobra.Command{
		Use:   "remove <run-id>",
		Short: "Remove a run from the queue",
		Long:  `Remove a queued run from the TeamCity queue.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc queue remove 12345
  tc queue remove 12345 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueueRemove(args[0], opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runQueueRemove(runID string, opts *queueRemoveOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	needsConfirmation := !opts.force && !NoInput && output.IsStdinTerminal()

	if needsConfirmation {
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Remove run %s from queue?", runID),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			output.Info("Cancelled")
			return nil
		}
	}

	if err := client.RemoveFromQueue(runID); err != nil {
		return fmt.Errorf("failed to remove run from queue: %w", err)
	}

	output.Success("Removed run %s from queue", runID)
	return nil
}

func newQueueTopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "top <run-id>",
		Short:   "Move a run to the top of the queue",
		Long:    `Move a queued run to the top of the queue, giving it highest priority.`,
		Args:    cobra.ExactArgs(1),
		Example: `  tc queue top 12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueueTop(args[0])
		},
	}

	return cmd
}

func runQueueTop(runID string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if err := client.MoveQueuedBuildToTop(runID); err != nil {
		return fmt.Errorf("failed to move run to top of queue: %w", err)
	}

	output.Success("Moved run %s to top of queue", runID)
	return nil
}

func newQueueApproveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "approve <run-id>",
		Short:   "Approve a queued run",
		Long:    `Approve a queued run that requires manual approval before it can run.`,
		Args:    cobra.ExactArgs(1),
		Example: `  tc queue approve 12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueueApprove(args[0])
		},
	}

	return cmd
}

func runQueueApprove(runID string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if err := client.ApproveQueuedBuild(runID); err != nil {
		return fmt.Errorf("failed to approve run: %w", err)
	}

	output.Success("Approved run %s", runID)
	return nil
}
