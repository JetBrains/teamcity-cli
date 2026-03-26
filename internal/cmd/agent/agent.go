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
		Long:  `List, view, and manage TeamCity build agents.`,
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
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
	limit      int
	jsonFields string
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
  teamcity agent list --json=id,name,connected,enabled`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentList(f, cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.pool, "pool", "p", "", "Filter by agent pool")
	cmd.Flags().BoolVar(&opts.connected, "connected", false, "Show only connected agents")
	cmd.Flags().BoolVar(&opts.enabled, "enabled", false, "Show only enabled agents")
	cmd.Flags().BoolVar(&opts.authorized, "authorized", false, "Show only authorized agents")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 100, "Maximum number of agents")
	cmdutil.AddJSONFieldsFlag(cmd, &opts.jsonFields)

	return cmd
}

func runAgentList(f *cmdutil.Factory, cmd *cobra.Command, opts *agentListOptions) error {
	if err := cmdutil.ValidateLimit(opts.limit); err != nil {
		return err
	}
	jsonResult, showHelp, err := cmdutil.ParseJSONFields(cmd, opts.jsonFields, &api.AgentFields)
	if err != nil {
		return err
	}
	if showHelp {
		return nil
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	agents, err := client.GetAgents(api.AgentsOptions{
		Pool:       opts.pool,
		Connected:  opts.connected,
		Enabled:    opts.enabled,
		Authorized: opts.authorized,
		Limit:      opts.limit,
		Fields:     jsonResult.Fields,
	})
	if err != nil {
		return err
	}

	if jsonResult.Enabled {
		return output.PrintJSON(agents)
	}

	if agents.Count == 0 {
		fmt.Println("No agents found")
		return nil
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

	output.AutoSizeColumns(headers, rows, 2, 1, 2)
	output.PrintTable(headers, rows)
	return nil
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
		return output.PrintJSON(agent)
	}

	fmt.Printf("%s\n", output.Cyan(agent.Name))
	fmt.Printf("ID: %d\n", agent.ID)

	if agent.Pool != nil {
		fmt.Printf("Pool: %s\n", agent.Pool.Name)
	}

	fmt.Printf("Status: %s\n", cmdutil.FormatAgentStatus(*agent))

	if agent.Connected {
		fmt.Printf("Connected: %s\n", output.Green("Yes"))
	} else {
		fmt.Printf("Connected: %s\n", output.Red("No"))
	}

	if agent.Enabled {
		fmt.Printf("Enabled: %s\n", output.Green("Yes"))
	} else {
		fmt.Printf("Enabled: %s\n", output.Faint("No"))
	}

	if agent.Authorized {
		fmt.Printf("Authorized: %s\n", output.Green("Yes"))
	} else {
		fmt.Printf("Authorized: %s\n", output.Yellow("No"))
	}

	if agent.Build != nil {
		fmt.Printf("\nCurrent build: %s %d  #%s (%s)\n",
			agent.Build.BuildType.Name,
			agent.Build.ID,
			agent.Build.Number,
			agent.Build.Status)
	}

	fmt.Printf("\n%s %s\n", output.Faint("View in browser:"), output.Green(agent.WebURL))

	if agent.Connected && agent.Authorized && agent.Enabled {
		fmt.Printf("%s teamcity agent term %d\n", output.Faint("Open terminal:"), agent.ID)
	}

	return nil
}
