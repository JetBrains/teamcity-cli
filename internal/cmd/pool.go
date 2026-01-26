package cmd

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func newPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage agent pools",
		Long:  `List agent pools and manage project assignments.`,
		Args:  cobra.NoArgs,
		RunE:  subcommandRequired,
	}

	cmd.AddCommand(newPoolListCmd())
	cmd.AddCommand(newPoolViewCmd())
	cmd.AddCommand(newPoolLinkCmd())
	cmd.AddCommand(newPoolUnlinkCmd())

	return cmd
}

type poolListOptions struct {
	jsonFields string
}

func newPoolListCmd() *cobra.Command {
	opts := &poolListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agent pools",
		Example: `  tc pool list
  tc pool list --json
  tc pool list --json=id,name,maxAgents`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPoolList(cmd, opts)
		},
	}

	AddJSONFieldsFlag(cmd, &opts.jsonFields)

	return cmd
}

func runPoolList(cmd *cobra.Command, opts *poolListOptions) error {
	jsonResult, showHelp, err := ParseJSONFields(cmd, opts.jsonFields, &api.PoolFields)
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

	pools, err := client.GetAgentPools()
	if err != nil {
		return err
	}

	if jsonResult.Enabled {
		return output.PrintJSON(pools)
	}

	if pools.Count == 0 {
		fmt.Println("No agent pools found")
		return nil
	}

	headers := []string{"ID", "NAME", "MAX AGENTS"}
	var rows [][]string

	for _, p := range pools.Pools {
		maxAgents := "unlimited"
		if p.MaxAgents > 0 {
			maxAgents = fmt.Sprintf("%d", p.MaxAgents)
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", p.ID),
			p.Name,
			maxAgents,
		})
	}

	output.PrintTable(headers, rows)
	return nil
}

func newPoolViewCmd() *cobra.Command {
	opts := &viewOptions{}

	cmd := &cobra.Command{
		Use:   "view <pool-id>",
		Short: "View pool details",
		Args:  cobra.ExactArgs(1),
		Example: `  tc pool view 0
  tc pool view 1 --web
  tc pool view 1 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0], "pool")
			if err != nil {
				return err
			}
			return runPoolView(id, opts)
		},
	}

	addViewFlags(cmd, opts)

	return cmd
}

func runPoolView(poolID int, opts *viewOptions) error {
	if opts.web {
		url := fmt.Sprintf("%s/agents.html?tab=agentPools&poolId=%d", config.GetServerURL(), poolID)
		return browser.OpenURL(url)
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	pool, err := client.GetAgentPool(poolID)
	if err != nil {
		return err
	}

	if opts.json {
		return output.PrintJSON(pool)
	}

	fmt.Printf("%s\n", output.Cyan(pool.Name))
	fmt.Printf("ID: %d\n", pool.ID)

	if pool.MaxAgents > 0 {
		fmt.Printf("Max Agents: %d\n", pool.MaxAgents)
	} else {
		fmt.Printf("Max Agents: %s\n", output.Faint("unlimited"))
	}

	if pool.Agents != nil && pool.Agents.Count > 0 {
		fmt.Printf("\n%s (%d)\n", output.Bold("Agents"), pool.Agents.Count)
		for _, a := range pool.Agents.Agents {
			status := formatAgentStatus(a)
			fmt.Printf("  %d  %s  %s\n", a.ID, a.Name, status)
		}
	} else {
		fmt.Printf("\n%s\n", output.Faint("No agents in this pool"))
	}

	if pool.Projects != nil && pool.Projects.Count > 0 {
		fmt.Printf("\n%s (%d)\n", output.Bold("Projects"), pool.Projects.Count)
		for _, p := range pool.Projects.Projects {
			fmt.Printf("  %s  %s\n", p.ID, p.Name)
		}
	} else {
		fmt.Printf("\n%s\n", output.Faint("No projects assigned to this pool"))
	}

	webURL := fmt.Sprintf("%s/agents.html?tab=agentPools&poolId=%d", config.GetServerURL(), poolID)
	fmt.Printf("\n%s %s\n", output.Faint("View in browser:"), output.Green(webURL))

	return nil
}

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

func newPoolProjectCmd(a poolProjectAction) *cobra.Command {
	return &cobra.Command{
		Use:     fmt.Sprintf("%s <pool-id> <project-id>", a.use),
		Short:   a.short,
		Long:    a.long,
		Args:    cobra.ExactArgs(2),
		Example: fmt.Sprintf("  tc pool %s 1 MyProject", a.use),
		RunE: func(cmd *cobra.Command, args []string) error {
			poolID, err := parseID(args[0], "pool")
			if err != nil {
				return err
			}
			client, err := getClient()
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

func newPoolLinkCmd() *cobra.Command   { return newPoolProjectCmd(poolProjectActions["link"]) }
func newPoolUnlinkCmd() *cobra.Command { return newPoolProjectCmd(poolProjectActions["unlink"]) }
