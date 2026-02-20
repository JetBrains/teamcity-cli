package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func newQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage build queue",
		Long:  `List and manage the TeamCity build queue.`,
		Args:  cobra.NoArgs,
		RunE:  subcommandRequired,
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
		Example: `  teamcity queue list
  teamcity queue list --job Falcon_Build
  teamcity queue list --json
  teamcity queue list --json=id,state,webUrl`,
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
	if err := validateLimit(opts.limit); err != nil {
		return err
	}
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

	for _, r := range queue.Builds {
		branch := r.BranchName
		if branch == "" {
			branch = "<default>"
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", r.ID),
			r.BuildTypeID,
			branch,
			r.State,
		})
	}

	output.AutoSizeColumns(headers, rows, 2, 1, 2)
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
		Example: `  teamcity queue remove 12345
  teamcity queue remove 12345 --force`,
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

type queueAction struct {
	use     string
	short   string
	long    string
	verb    string
	execute func(api.ClientInterface, string) error
}

var queueActions = map[string]queueAction{
	"top": {"top", "Move a run to the top of the queue",
		"Move a queued run to the top of the queue, giving it highest priority.",
		"Moved run %s to top of queue",
		func(c api.ClientInterface, id string) error { return c.MoveQueuedBuildToTop(id) }},
	"approve": {"approve", "Approve a queued run",
		"Approve a queued run that requires manual approval before it can run.",
		"Approved run %s",
		func(c api.ClientInterface, id string) error { return c.ApproveQueuedBuild(id) }},
}

func newQueueActionCmd(a queueAction) *cobra.Command {
	return &cobra.Command{
		Use:     fmt.Sprintf("%s <run-id>", a.use),
		Short:   a.short,
		Long:    a.long,
		Args:    cobra.ExactArgs(1),
		Example: fmt.Sprintf("  teamcity queue %s 12345", a.use),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}
			if err := a.execute(client, args[0]); err != nil {
				return fmt.Errorf("failed to %s run: %w", a.use, err)
			}
			output.Success(a.verb, args[0])
			return nil
		},
	}
}

func newQueueTopCmd() *cobra.Command     { return newQueueActionCmd(queueActions["top"]) }
func newQueueApproveCmd() *cobra.Command { return newQueueActionCmd(queueActions["approve"]) }
