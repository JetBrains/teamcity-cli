package pool

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type poolProjectAction struct {
	use     string
	short   string
	long    string
	verb    string
	execute func(api.ClientInterface, int, string) error
}

var poolProjectActions = map[string]poolProjectAction{
	"link": {
		use:   "link",
		short: "Link a project to an agent pool",
		long:  "Link a project to an agent pool, allowing the project's builds to run on agents in that pool.",
		verb:  "Linked",
		execute: func(c api.ClientInterface, poolID int, projectID string) error {
			return c.AddProjectToPool(poolID, projectID)
		},
	},
	"unlink": {
		use:   "unlink",
		short: "Unlink a project from an agent pool",
		long:  "Unlink a project from an agent pool, removing the project's access to agents in that pool.",
		verb:  "Unlinked",
		execute: func(c api.ClientInterface, poolID int, projectID string) error {
			return c.RemoveProjectFromPool(poolID, projectID)
		},
	},
}

func newPoolProjectCmd(f *cmdutil.Factory, a poolProjectAction) *cobra.Command {
	return &cobra.Command{
		Use:     fmt.Sprintf("%s <pool-id> <project-id>", a.use),
		Short:   a.short,
		Long:    a.long,
		Args:    cobra.ExactArgs(2),
		Example: fmt.Sprintf("  teamcity pool %s 1 MyProject", a.use),
		RunE: func(cmd *cobra.Command, args []string) error {
			poolID, err := cmdutil.ParseID(args[0], "pool")
			if err != nil {
				return err
			}
			client, err := f.Client()
			if err != nil {
				return err
			}
			if err := a.execute(client, poolID, args[1]); err != nil {
				return fmt.Errorf("failed to %s project: %w", a.use, err)
			}
			output.Success("%s project %s to pool %d", a.verb, args[1], poolID)
			return nil
		},
	}
}

func newPoolLinkCmd(f *cmdutil.Factory) *cobra.Command   { return newPoolProjectCmd(f, poolProjectActions["link"]) }
func newPoolUnlinkCmd(f *cmdutil.Factory) *cobra.Command { return newPoolProjectCmd(f, poolProjectActions["unlink"]) }
