package test

import (
	"fmt"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type resolveOptions struct {
	project string
	job     string
	state   string
	json    bool
}

func newResolveCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &resolveOptions{}

	cmd := &cobra.Command{
		Use:   "resolve <test>",
		Short: "Close a test's investigation in a project or job",
		Long: `Resolve the active investigation(s) for a test within a project or build configuration.

A scope is required — pass --project or --job. The test name is resolved to its
id; the matching investigation in that scope is then closed.

--state controls how it is closed:
  fixed       the test was fixed (default)
  given-up    the investigation is abandoned`,
		Example: `  teamcity test resolve com.example.FooTest.flaky --project Falcon
  teamcity test resolve com.example.FooTest.flaky --job Falcon_Build --state given-up`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResolve(f, cmd, opts, args[0])
		},
	}

	cmd.Flags().StringVar(&opts.project, "project", "", "Project the test is investigated in")
	cmd.Flags().StringVar(&opts.job, "job", "", "Job the test is investigated in (takes precedence over --project)")
	cmd.Flags().StringVar(&opts.state, "state", "fixed", "How to close the investigation: fixed | given-up")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output the resolution as JSON")

	return cmd
}

func runResolve(f *cmdutil.Factory, cmd *cobra.Command, opts *resolveOptions, name string) error {
	p := f.Printer

	state, err := parseResolveState(opts.state)
	if err != nil {
		return err
	}

	scope, err := resolveScope(f, cmd, opts.project, opts.job)
	if err != nil {
		return err
	}

	f.Analytics.Track(analytics.GroupTest, analytics.EventTestInvestigated, map[string]any{
		"action":      analytics.TestActionResolve,
		"is_from_job": scope.Job != "",
	})

	client, err := f.Client()
	if err != nil {
		return err
	}

	testID, err := resolveTestID(f.Context(), p, client, name, scope)
	if err != nil {
		return err
	}

	if err := client.ResolveInvestigation(f.Context(), testID, scope, state); err != nil {
		return err
	}

	if opts.json {
		return p.PrintJSON(map[string]any{"test": name, "state": state, "resolved": true})
	}

	p.Success("Resolved %s (%s)", name, state)
	return nil
}

// parseResolveState maps the --state flag value to a TeamCity investigation state.
//
//	fixed (or empty) → FIXED
//	given-up         → GIVEN_UP
func parseResolveState(state string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "", "fixed":
		return "FIXED", nil
	case "given-up", "givenup", "given_up":
		return "GIVEN_UP", nil
	}
	return "", api.Validation(
		fmt.Sprintf("invalid --state value %q", state),
		"use 'fixed' or 'given-up'",
	)
}
