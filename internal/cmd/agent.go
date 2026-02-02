package cmd

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage build agents",
		Long:  `List, view, and manage TeamCity build agents.`,
		Args:  cobra.NoArgs,
		RunE:  subcommandRequired,
	}

	cmd.AddCommand(newAgentListCmd())
	cmd.AddCommand(newAgentViewCmd())
	cmd.AddCommand(newAgentJobsCmd())
	cmd.AddCommand(newAgentMoveCmd())
	cmd.AddCommand(newAgentEnableCmd())
	cmd.AddCommand(newAgentDisableCmd())
	cmd.AddCommand(newAgentAuthorizeCmd())
	cmd.AddCommand(newAgentDeauthorizeCmd())
	cmd.AddCommand(newAgentTerminalCmd())
	cmd.AddCommand(newAgentExecCmd())

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

func newAgentListCmd() *cobra.Command {
	opts := &agentListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List build agents",
		Example: `  tc agent list
  tc agent list --pool Default
  tc agent list --connected
  tc agent list --json
  tc agent list --json=id,name,connected,enabled`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentList(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.pool, "pool", "p", "", "Filter by agent pool")
	cmd.Flags().BoolVar(&opts.connected, "connected", false, "Show only connected agents")
	cmd.Flags().BoolVar(&opts.enabled, "enabled", false, "Show only enabled agents")
	cmd.Flags().BoolVar(&opts.authorized, "authorized", false, "Show only authorized agents")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 100, "Maximum number of agents")
	AddJSONFieldsFlag(cmd, &opts.jsonFields)

	return cmd
}

func runAgentList(cmd *cobra.Command, opts *agentListOptions) error {
	jsonResult, showHelp, err := ParseJSONFields(cmd, opts.jsonFields, &api.AgentFields)
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

	widths := output.ColumnWidths(20, 40, 40, 10)

	for _, a := range agents.Agents {
		status := formatAgentStatus(a)
		poolName := ""
		if a.Pool != nil {
			poolName = a.Pool.Name
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", a.ID),
			output.Truncate(a.Name, widths[0]),
			output.Truncate(poolName, widths[1]),
			status,
		})
	}

	output.PrintTable(headers, rows)
	return nil
}

func formatAgentStatus(a api.Agent) string {
	if !a.Authorized {
		return output.Yellow("Unauthorized")
	}
	if !a.Enabled {
		return output.Faint("Disabled")
	}
	if !a.Connected {
		return output.Red("Disconnected")
	}
	return output.Green("Connected")
}

func newAgentViewCmd() *cobra.Command {
	opts := &viewOptions{}
	cmd := &cobra.Command{
		Use:   "view <agent>",
		Short: "View agent details",
		Args:  cobra.ExactArgs(1),
		Example: `  tc agent view 1
  tc agent view Agent-Linux-01
  tc agent view Agent-Linux-01 --web
  tc agent view 1 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentView(args[0], opts)
		},
	}
	addViewFlags(cmd, opts)
	return cmd
}

func runAgentView(nameOrID string, opts *viewOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	agent, err := resolveAgent(client, nameOrID)
	if err != nil {
		return err
	}

	if opts.web {
		return browser.OpenURL(agent.WebURL)
	}

	if opts.json {
		return output.PrintJSON(agent)
	}

	fmt.Printf("%s\n", output.Cyan(agent.Name))
	fmt.Printf("ID: %d\n", agent.ID)

	if agent.Pool != nil {
		fmt.Printf("Pool: %s\n", agent.Pool.Name)
	}

	fmt.Printf("Status: %s\n", formatAgentStatus(*agent))

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
		fmt.Printf("\nCurrent build: %s #%s (%s)\n",
			agent.Build.BuildType.Name,
			agent.Build.Number,
			agent.Build.Status)
	}

	fmt.Printf("\n%s %s\n", output.Faint("View in browser:"), output.Green(agent.WebURL))

	if agent.Connected && agent.Authorized && agent.Enabled {
		fmt.Printf("%s tc agent term %d\n", output.Faint("Open terminal:"), agent.ID)
	}

	return nil
}

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

func newAgentActionCmd(a agentAction) *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s <agent>", a.use),
		Short: a.short,
		Long:  a.long,
		Args:  cobra.ExactArgs(1),
		Example: fmt.Sprintf(`  tc agent %s 1
  tc agent %s Agent-Linux-01`, a.use, a.use),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}
			agentID, agentName, err := resolveAgentID(client, args[0])
			if err != nil {
				return err
			}
			if err := a.execute(client, agentID); err != nil {
				return fmt.Errorf("failed to %s agent: %w", a.use, err)
			}
			output.Success("%s agent %s", a.verb, agentName)
			return nil
		},
	}
}

func newAgentEnableCmd() *cobra.Command      { return newAgentActionCmd(agentActions["enable"]) }
func newAgentDisableCmd() *cobra.Command     { return newAgentActionCmd(agentActions["disable"]) }
func newAgentAuthorizeCmd() *cobra.Command   { return newAgentActionCmd(agentActions["authorize"]) }
func newAgentDeauthorizeCmd() *cobra.Command { return newAgentActionCmd(agentActions["deauthorize"]) }

type agentJobsOptions struct {
	incompatible bool
	json         bool
}

func newAgentJobsCmd() *cobra.Command {
	opts := &agentJobsOptions{}

	cmd := &cobra.Command{
		Use:   "jobs <agent>",
		Short: "Show jobs an agent can run",
		Long:  `List build configurations (jobs) that are compatible or incompatible with an agent.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc agent jobs 1
  tc agent jobs Agent-Linux-01
  tc agent jobs Agent-Linux-01 --incompatible
  tc agent jobs 1 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentJobs(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.incompatible, "incompatible", false, "Show incompatible jobs with reasons")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runAgentJobs(nameOrID string, opts *agentJobsOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	agentID, _, err := resolveAgentID(client, nameOrID)
	if err != nil {
		return err
	}

	if opts.incompatible {
		return showIncompatibleJobs(client, agentID, opts.json)
	}
	return showCompatibleJobs(client, agentID, opts.json)
}

func showCompatibleJobs(client api.ClientInterface, agentID int, jsonOutput bool) error {
	jobs, err := client.GetAgentCompatibleBuildTypes(agentID)
	if err != nil {
		return err
	}

	if jsonOutput {
		return output.PrintJSON(jobs)
	}

	if jobs.Count == 0 {
		fmt.Println("No compatible jobs found")
		return nil
	}

	fmt.Printf("%s (%d)\n\n", output.Green("Compatible Jobs"), jobs.Count)

	headers := []string{"ID", "NAME", "PROJECT"}
	var rows [][]string

	widths := output.ColumnWidths(20, 40, 20, 40, 30)

	for _, j := range jobs.BuildTypes {
		rows = append(rows, []string{
			output.Truncate(j.ID, widths[0]),
			output.Truncate(j.Name, widths[1]),
			output.Truncate(j.ProjectName, widths[2]),
		})
	}

	output.PrintTable(headers, rows)
	return nil
}

func showIncompatibleJobs(client api.ClientInterface, agentID int, jsonOutput bool) error {
	compat, err := client.GetAgentIncompatibleBuildTypes(agentID)
	if err != nil {
		return err
	}

	if jsonOutput {
		return output.PrintJSON(compat)
	}

	if compat.Count == 0 {
		fmt.Println("No incompatible jobs found")
		return nil
	}

	fmt.Printf("%s (%d)\n\n", output.Yellow("Incompatible Jobs"), compat.Count)

	for _, c := range compat.Compatibility {
		if c.BuildType == nil {
			continue
		}
		fmt.Printf("%s %s\n", output.Cyan(c.BuildType.Name), output.Faint("("+c.BuildType.ID+")"))
		if c.BuildType.ProjectName != "" {
			fmt.Printf("  Project: %s\n", c.BuildType.ProjectName)
		}
		if c.Reasons != nil && len(c.Reasons.Reasons) > 0 {
			fmt.Printf("  Reasons:\n")
			for _, reason := range c.Reasons.Reasons {
				fmt.Printf("    %s %s\n", output.Red("•"), reason)
			}
		}
		fmt.Println()
	}

	return nil
}

func newAgentMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <agent> <pool-id>",
		Short: "Move an agent to a different pool",
		Long:  `Move an agent to a different agent pool.`,
		Args:  cobra.ExactArgs(2),
		Example: `  tc agent move 1 0
  tc agent move Agent-Linux-01 2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			poolID, err := parseID(args[1], "pool")
			if err != nil {
				return err
			}
			return runAgentMove(args[0], poolID)
		},
	}

	return cmd
}

func runAgentMove(nameOrID string, poolID int) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	agentID, agentName, err := resolveAgentID(client, nameOrID)
	if err != nil {
		return err
	}

	if err := client.SetAgentPool(agentID, poolID); err != nil {
		return fmt.Errorf("failed to move agent: %w", err)
	}

	output.Success("Moved agent %s to pool %d", agentName, poolID)
	return nil
}
	return nil
}
