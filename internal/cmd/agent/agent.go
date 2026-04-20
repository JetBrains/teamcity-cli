package agent

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage build agents",
		Long: `List, view, and manage TeamCity build agents.

Build agents run the jobs assigned by the TeamCity server. Use these
commands to inspect agent state, toggle availability, and open a shell
to a running agent.

See: https://www.jetbrains.com/help/teamcity/build-agent.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newAgentListCmd(f))
	cmd.AddCommand(newAgentViewCmd(f))
	cmd.AddCommand(newAgentJobsCmd(f))
	cmd.AddCommand(newAgentMoveCmd(f))
	cmd.AddCommand(newAgentActionCmd(f, agentActions["enable"]))
	cmd.AddCommand(newAgentActionCmd(f, agentActions["disable"]))
	cmd.AddCommand(newAgentActionCmd(f, agentActions["authorize"]))
	cmd.AddCommand(newAgentActionCmd(f, agentActions["deauthorize"]))
	cmd.AddCommand(newAgentTerminalCmd(f))
	cmd.AddCommand(newAgentExecCmd(f))
	cmd.AddCommand(newAgentRebootCmd(f))

	return cmd
}

type agentListOptions struct {
	pool       string
	connected  bool
	enabled    bool
	authorized bool
	cmdutil.ListFlags
}

func newAgentListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &agentListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List build agents",
		Aliases: []string{"ls"},
		Example: `  teamcity agent list
  teamcity agent list --pool Default
  teamcity agent list --connected
  teamcity agent list --json
  teamcity agent list --json=id,name,connected,enabled
  teamcity agent list --plain
  teamcity agent list --plain --no-header`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.AgentFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.pool, "pool", "p", "", "Filter by agent pool")
	cmd.Flags().BoolVar(&opts.connected, "connected", false, "Show only connected agents")
	cmd.Flags().BoolVar(&opts.enabled, "enabled", false, "Show only enabled agents")
	cmd.Flags().BoolVar(&opts.authorized, "authorized", false, "Show only authorized agents")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 100)

	return cmd
}

func (opts *agentListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	agents, err := client.GetAgents(api.AgentsOptions{
		Pool:       opts.pool,
		Connected:  opts.connected,
		Enabled:    opts.enabled,
		Authorized: opts.authorized,
		Limit:      opts.Limit,
		Fields:     fields,
	})
	if err != nil {
		return nil, err
	}

	headers := []string{"ID", "NAME", "POOL", "STATUS"}
	var rows [][]string

	for _, a := range agents.Agents {
		status := cmdutil.FormatAgentStatus(a)
		poolName := ""
		if a.Pool != nil {
			poolName = a.Pool.Name
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", a.ID),
			a.Name,
			poolName,
			status,
		})
	}

	return &cmdutil.ListResult{
		JSON:     agents,
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{1, 2}},
		EmptyMsg: "No agents found",
		EmptyTip: output.TipNoAgents,
	}, nil
}

func newAgentViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cmdutil.ViewOptions{}
	cmd := &cobra.Command{
		Use:     "view <agent>",
		Short:   "View agent details",
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity agent view 1
  teamcity agent view Agent-Linux-01
  teamcity agent view Agent-Linux-01 --web
  teamcity agent view 1 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentView(f, args[0], opts)
		},
	}
	cmdutil.AddViewFlags(cmd, opts)
	return cmd
}

func runAgentView(f *cmdutil.Factory, nameOrID string, opts *cmdutil.ViewOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	agent, err := cmdutil.ResolveAgent(client, nameOrID)
	if err != nil {
		return err
	}

	if opts.Web {
		return browser.OpenURL(agent.WebURL)
	}

	if opts.JSON {
		return f.Printer.PrintJSON(agent)
	}

	p := f.Printer
	_, _ = fmt.Fprintf(p.Out, "%s\n", output.Cyan(agent.Name))
	_, _ = fmt.Fprintf(p.Out, "ID: %d\n", agent.ID)

	if agent.Pool != nil {
		_, _ = fmt.Fprintf(p.Out, "Pool: %s\n", agent.Pool.Name)
	}

	_, _ = fmt.Fprintf(p.Out, "Status: %s\n", cmdutil.FormatAgentStatus(*agent))

	if agent.Connected {
		_, _ = fmt.Fprintf(p.Out, "Connected: %s\n", output.Green("Yes"))
	} else {
		_, _ = fmt.Fprintf(p.Out, "Connected: %s\n", output.Red("No"))
	}

	if agent.Enabled {
		_, _ = fmt.Fprintf(p.Out, "Enabled: %s\n", output.Green("Yes"))
	} else {
		_, _ = fmt.Fprintf(p.Out, "Enabled: %s\n", output.Faint("No"))
	}

	if agent.Authorized {
		_, _ = fmt.Fprintf(p.Out, "Authorized: %s\n", output.Green("Yes"))
	} else {
		_, _ = fmt.Fprintf(p.Out, "Authorized: %s\n", output.Yellow("No"))
	}

	if agent.Build != nil {
		_, _ = fmt.Fprintf(p.Out, "\nCurrent build: %s %d  #%s (%s)\n",
			agent.Build.BuildType.Name,
			agent.Build.ID,
			agent.Build.Number,
			agent.Build.Status)
	}

	_, _ = fmt.Fprintf(p.Out, "\n%s %s\n", output.Faint("View in browser:"), output.Green(agent.WebURL))

	if agent.Connected && agent.Authorized && agent.Enabled {
		_, _ = fmt.Fprintf(p.Out, "%s teamcity agent term %d\n", output.Faint("Open terminal:"), agent.ID)
	}

	return nil
}
