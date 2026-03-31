package agent

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type agentJobsOptions struct {
	incompatible bool
	json         bool
	plain        bool
	noHeader     bool
}

func newAgentJobsCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &agentJobsOptions{}

	cmd := &cobra.Command{
		Use:   "jobs <agent>",
		Short: "Show jobs an agent can run",
		Long:  `List build configurations (jobs) that are compatible or incompatible with an agent.`,
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity agent jobs 1
  teamcity agent jobs Agent-Linux-01
  teamcity agent jobs Agent-Linux-01 --incompatible
  teamcity agent jobs 1 --json
  teamcity agent jobs 1 --plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentJobs(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.incompatible, "incompatible", false, "Show incompatible jobs with reasons")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "Output in plain text format for scripting")
	cmd.Flags().BoolVar(&opts.noHeader, "no-header", false, "Omit header row (use with --plain)")
	cmd.MarkFlagsMutuallyExclusive("json", "plain")

	return cmd
}

func runAgentJobs(f *cmdutil.Factory, nameOrID string, opts *agentJobsOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	agentID, _, err := cmdutil.ResolveAgentID(client, nameOrID)
	if err != nil {
		return err
	}

	p := f.Printer
	if opts.incompatible {
		return showIncompatibleJobs(p, client, agentID, opts.json)
	}
	return showCompatibleJobs(p, client, agentID, opts)
}

func showCompatibleJobs(p *output.Printer, client api.ClientInterface, agentID int, opts *agentJobsOptions) error {
	jobs, err := client.GetAgentCompatibleBuildTypes(agentID)
	if err != nil {
		return err
	}

	if opts.json {
		return p.PrintJSON(jobs)
	}

	if jobs.Count == 0 {
		_, _ = fmt.Fprintln(p.Out, "No compatible jobs found")
		return nil
	}

	if !opts.plain {
		_, _ = fmt.Fprintf(p.Out, "%s (%d)\n\n", output.Green("Compatible Jobs"), jobs.Count)
	}

	headers := []string{"ID", "NAME", "PROJECT"}
	var rows [][]string

	for _, j := range jobs.BuildTypes {
		rows = append(rows, []string{
			j.ID,
			j.Name,
			j.ProjectName,
		})
	}

	if opts.plain {
		p.PrintPlainTable(headers, rows, opts.noHeader)
	} else {
		output.AutoSizeColumns(headers, rows, 2, 0, 1, 2)
		p.PrintTable(headers, rows)
	}
	return nil
}

func showIncompatibleJobs(p *output.Printer, client api.ClientInterface, agentID int, jsonOutput bool) error {
	compat, err := client.GetAgentIncompatibleBuildTypes(agentID)
	if err != nil {
		return err
	}

	if jsonOutput {
		return p.PrintJSON(compat)
	}

	if compat.Count == 0 {
		_, _ = fmt.Fprintln(p.Out, "No incompatible jobs found")
		return nil
	}

	_, _ = fmt.Fprintf(p.Out, "%s (%d)\n\n", output.Yellow("Incompatible Jobs"), compat.Count)

	for _, c := range compat.Compatibility {
		if c.BuildType == nil {
			continue
		}
		_, _ = fmt.Fprintf(p.Out, "%s %s\n", output.Cyan(c.BuildType.Name), output.Faint("("+c.BuildType.ID+")"))
		if c.BuildType.ProjectName != "" {
			_, _ = fmt.Fprintf(p.Out, "  Project: %s\n", c.BuildType.ProjectName)
		}
		if c.Reasons != nil && len(c.Reasons.Reasons) > 0 {
			_, _ = fmt.Fprintf(p.Out, "  Reasons:\n")
			for _, reason := range c.Reasons.Reasons {
				_, _ = fmt.Fprintf(p.Out, "    %s %s\n", output.Red("•"), reason)
			}
		}
		_, _ = fmt.Fprintln(p.Out)
	}

	return nil
}
