package pool

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func newPoolListCmd(f *cmdutil.Factory) *cobra.Command {
	flags := &cmdutil.ListFlags{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List agent pools",
		Aliases: []string{"ls"},
		Example: `  teamcity pool list
  teamcity pool list --json
  teamcity pool list --json=id,name,maxAgents`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, flags, &api.PoolFields, fetchPools)
		},
	}

	cmdutil.AddJSONFieldsFlag(cmd, &flags.JSONFields)

	return cmd
}

func fetchPools(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	pools, err := client.GetAgentPools(fields)
	if err != nil {
		return nil, err
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

	return &cmdutil.ListResult{
		JSON:     pools,
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows},
		EmptyMsg: "No agent pools found",
	}, nil
}

func newPoolViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cmdutil.ViewOptions{}

	cmd := &cobra.Command{
		Use:     "view <pool-id>",
		Short:   "View pool details",
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity pool view 0
  teamcity pool view 1 --web
  teamcity pool view 1 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmdutil.ParseID(args[0], "pool")
			if err != nil {
				return err
			}
			return runPoolView(f, id, opts)
		},
	}

	cmdutil.AddViewFlags(cmd, opts)

	return cmd
}

func runPoolView(f *cmdutil.Factory, poolID int, opts *cmdutil.ViewOptions) error {
	if opts.Web {
		url := fmt.Sprintf("%s/agents.html?tab=agentPools&poolId=%d", config.GetServerURL(), poolID)
		return browser.OpenURL(url)
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	pool, err := client.GetAgentPool(poolID)
	if err != nil {
		return err
	}

	if opts.JSON {
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
			status := cmdutil.FormatAgentStatus(a)
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
