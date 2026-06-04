package test

import (
	"context"
	"errors"
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

// resolveScope turns the --project/--job flags into a mute/investigation scope and the owning
// project id used for name→id resolution. A scope is required — server-wide writes are rejected.
func resolveScope(f *cmdutil.Factory, cmd *cobra.Command, project, job string) (api.ProblemScopeOptions, string, error) {
	p := f.ResolveProject(project)
	j := f.ResolveDefaultJob(job)

	// An explicit --project without --job means project-wide; don't let a linked job override it.
	if cmd.Flags().Changed("project") && !cmd.Flags().Changed("job") {
		j = ""
	}

	if p == "" && j == "" {
		return api.ProblemScopeOptions{}, "", api.Validation(
			"a scope is required",
			"pass --project or --job (server-wide writes are not allowed)",
		)
	}
	return api.ProblemScopeOptions{Project: p, Job: j}, p, nil
}

// resolveTestID resolves a test name to its id, printing the candidate list and returning a
// clean validation error when the name is ambiguous (no action is taken in that case).
func resolveTestID(ctx context.Context, p *output.Printer, client api.ClientInterface, name, projectID string) (string, error) {
	id, err := client.ResolveTestID(ctx, name, projectID)
	if err == nil {
		return id, nil
	}

	var ambig *api.AmbiguousTestError
	if errors.As(err, &ambig) {
		printCandidates(p, ambig)
		return "", api.Validation(
			fmt.Sprintf("test name %q matches %d tests", ambig.Name, len(ambig.Candidates)),
			"narrow the scope with --job, or use a fully-qualified test name",
		)
	}
	return "", err
}

// printCandidates renders the ambiguous-match candidates as an id/name table.
func printCandidates(p *output.Printer, ambig *api.AmbiguousTestError) {
	p.Warn("%q matches %d tests:", ambig.Name, len(ambig.Candidates))
	headers := []string{"ID", "NAME"}
	rows := make([][]string, 0, len(ambig.Candidates))
	for _, c := range ambig.Candidates {
		rows = append(rows, []string{c.ID, c.Name})
	}
	output.AutoSizeColumns(headers, rows, 2, 1)
	p.PrintTable(headers, rows)
}
