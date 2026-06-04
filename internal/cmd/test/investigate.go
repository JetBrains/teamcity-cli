package test

import (
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type investigateOptions struct {
	project  string
	job      string
	assignee string
	json     bool
}

func newInvestigateCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &investigateOptions{}

	cmd := &cobra.Command{
		Use:   "investigate <test>",
		Short: "Assign an investigation for a test in a project or job",
		Long: `Open an investigation (state TAKEN) for a test within a project or build configuration.

A scope is required — pass --project or --job. The test name is resolved to its
id; an ambiguous name prints the matching candidates and exits without acting.

Use --assignee to assign the investigation to a specific user; otherwise it is
recorded without an assignee. Resolve it later with 'teamcity test resolve'.

This change is reversible, so there is no confirmation prompt.

See: https://www.jetbrains.com/help/teamcity/investigating-and-muting-build-failures.html`,
		Example: `  teamcity test investigate com.example.FooTest.flaky --project Falcon
  teamcity test investigate com.example.FooTest.flaky --job Falcon_Build --assignee jdoe`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInvestigate(f, cmd, opts, args[0])
		},
	}

	cmd.Flags().StringVar(&opts.project, "project", "", "Project to investigate the test in")
	cmd.Flags().StringVar(&opts.job, "job", "", "Job to investigate the test in (takes precedence over --project)")
	cmd.Flags().StringVar(&opts.assignee, "assignee", "", "Username to assign the investigation to")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output the created investigation as JSON")

	return cmd
}

func runInvestigate(f *cmdutil.Factory, cmd *cobra.Command, opts *investigateOptions, name string) error {
	p := f.Printer

	scope, projectID, err := resolveScope(f, cmd, opts.project, opts.job)
	if err != nil {
		return err
	}

	f.Analytics.Track(analytics.GroupTest, analytics.EventTestInvestigated, map[string]any{
		"action":       analytics.TestActionInvestigate,
		"is_from_job":  scope.Job != "",
		"has_assignee": opts.assignee != "",
	})

	client, err := f.Client()
	if err != nil {
		return err
	}

	testID, err := resolveTestID(f.Context(), p, client, name, projectID)
	if err != nil {
		return err
	}

	inv, err := client.CreateInvestigation(f.Context(), testID, scope, opts.assignee)
	if err != nil {
		return err
	}

	if opts.json {
		return p.PrintJSON(inv)
	}

	if opts.assignee != "" {
		p.Success("Investigating %s (assigned to %s)", name, opts.assignee)
	} else {
		p.Success("Investigating %s", name)
	}
	return nil
}
