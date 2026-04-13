package queue

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

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

func newQueueActionCmd(f *cmdutil.Factory, a queueAction) *cobra.Command {
	return &cobra.Command{
		Use:     fmt.Sprintf("%s <id>", a.use),
		Short:   a.short,
		Long:    a.long,
		Args:    cobra.ExactArgs(1),
		Example: fmt.Sprintf("  teamcity queue %s 12345", a.use),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.Client()
			if err != nil {
				return err
			}
			if err := a.execute(client, args[0]); err != nil {
				return fmt.Errorf("failed to %s run: %w", a.use, err)
			}
			f.Printer.Success(a.verb, args[0])
			return nil
		},
	}
}

func newQueueTopCmd(f *cmdutil.Factory) *cobra.Command {
	return newQueueActionCmd(f, queueActions["top"])
}
func newQueueApproveCmd(f *cmdutil.Factory) *cobra.Command {
	return newQueueActionCmd(f, queueActions["approve"])
}
