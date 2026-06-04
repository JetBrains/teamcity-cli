package agent

import (
	"fmt"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type agentJobsOptions struct {
	incompatible bool
	cmdutil.ListOptions
}

func newAgentJobsCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &agentJobsOptions{}

	cmd := &cobra.Command{
		Use:   "jobs <agent>",
		Short: "Show jobs an agent can run",
		Long: `Show jobs that are compatible with an agent.

Pass --incompatible to show jobs that would not run on this agent,
with the requirement reasons (missing parameters, unmet tool
versions, pool restrictions, etc.).`,
		Args: cobra.ExactArgs(1),
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
	opts.AddFlags(cmd, false)

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
		return showIncompatibleJobs(p, client, agentID, opts)
	}
	return showCompatibleJobs(p, client, agentID, opts)
}

func showCompatibleJobs(p *output.Printer, client api.ClientInterface, agentID int, opts *agentJobsOptions) error {
	jobs, err := client.GetAgentCompatibleBuildTypes(agentID)
	if err != nil {
		return err
	}

	if opts.JSON {
		return p.PrintJSON(jobs)
	}

	if jobs.Count == 0 {
		_, _ = fmt.Fprintln(p.Out, "No compatible jobs found")
		return nil
	}

	if !opts.Plain {
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

	if opts.Plain {
		p.PrintPlainTable(headers, rows, opts.NoHeader)
	} else {
		output.AutoSizeColumns(headers, rows, 2, 0, 1, 2)
		p.PrintTable(headers, rows)
	}
	return nil
}

func showIncompatibleJobs(p *output.Printer, client api.ClientInterface, agentID int, opts *agentJobsOptions) error {
	compat, err := client.GetAgentIncompatibleBuildTypes(agentID)
	if err != nil {
		return err
	}

	if opts.JSON {
		return p.PrintJSON(compat)
	}

	if compat.Count == 0 {
		_, _ = fmt.Fprintln(p.Out, "No incompatible jobs found")
		return nil
	}

	if opts.Plain {
		return showIncompatibleJobsPlain(p, compat, opts.NoHeader)
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
				_, _ = fmt.Fprintf(p.Out, "    %s %s\n", output.Red(output.Sym().Bullet), reason)
			}
		}
		_, _ = fmt.Fprintln(p.Out)
	}

	return nil
}

func showIncompatibleJobsPlain(p *output.Printer, compat *api.CompatibilityList, noHeader bool) error {
	headers := []string{"ID", "NAME", "PROJECT", "REASONS"}
	var rows [][]string

	for _, c := range compat.Compatibility {
		if c.BuildType == nil {
			continue
		}
		reasons := ""
		if c.Reasons != nil {
			reasons = strings.Join(c.Reasons.Reasons, "; ")
		}
		rows = append(rows, []string{
			c.BuildType.ID,
			c.BuildType.Name,
			c.BuildType.ProjectName,
			reasons,
		})
	}

	p.PrintPlainTable(headers, rows, noHeader)
	return nil
}
