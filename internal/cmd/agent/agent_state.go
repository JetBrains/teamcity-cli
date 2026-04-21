package agent

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type agentAction struct {
	use     string
	short   string
	long    string
	verb    string
	execute func(api.ClientInterface, int) error
}

var agentActions = map[string]agentAction{
	"enable": {"enable", "Enable an agent", "Enable an agent to allow it to run builds.", "Enabled",
		func(c api.ClientInterface, id int) error { return c.EnableAgent(id, true) }},
	"disable": {"disable", "Disable an agent", "Disable an agent to prevent it from running builds.", "Disabled",
		func(c api.ClientInterface, id int) error { return c.EnableAgent(id, false) }},
	"authorize": {"authorize", "Authorize an agent", "Authorize an agent to allow it to connect and run builds.", "Authorized",
		func(c api.ClientInterface, id int) error { return c.AuthorizeAgent(id, true) }},
	"deauthorize": {"deauthorize", "Deauthorize an agent", "Deauthorize an agent to revoke its permission to connect.", "Deauthorized",
		func(c api.ClientInterface, id int) error { return c.AuthorizeAgent(id, false) }},
}

func newAgentActionCmd(f *cmdutil.Factory, a agentAction) *cobra.Command {
	return &cobra.Command{
		Use:   a.use + " <agent>",
		Short: a.short,
		Long:  a.long,
		Args:  cobra.ExactArgs(1),
		Example: fmt.Sprintf(`  teamcity agent %s 1
  teamcity agent %s Agent-Linux-01`, a.use, a.use),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.Client()
			if err != nil {
				return err
			}
			agentID, agentName, err := cmdutil.ResolveAgentID(client, args[0])
			if err != nil {
				return err
			}
			if err := a.execute(client, agentID); err != nil {
				return fmt.Errorf("failed to %s agent: %w", a.use, err)
			}
			f.Analytics.Track(analytics.GroupAgent, analytics.EventStateChanged, map[string]any{"action": a.use})
			f.Printer.Success("%s agent %s", a.verb, agentName)
			return nil
		},
	}
}
