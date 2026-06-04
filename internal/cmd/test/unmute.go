package test

import (
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type unmuteOptions struct {
	project string
	job     string
	json    bool
}

func newUnmuteCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &unmuteOptions{}

	cmd := &cobra.Command{
		Use:   "unmute <test>",
		Short: "Remove a test's mute in a project or job",
		Long: `Remove the active mute(s) for a test within a project or build configuration.

A scope is required — pass --project or --job. The test name is resolved to its
id; the matching mute in that scope is then deleted.`,
		Example: `  teamcity test unmute com.example.FooTest.flaky --project Falcon
  teamcity test unmute com.example.FooTest.flaky --job Falcon_Build`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnmute(f, cmd, opts, args[0])
		},
	}

	cmd.Flags().StringVar(&opts.project, "project", "", "Project the test is muted in")
	cmd.Flags().StringVar(&opts.job, "job", "", "Job the test is muted in (takes precedence over --project)")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output the removed mutes as JSON")

	return cmd
}

func runUnmute(f *cmdutil.Factory, cmd *cobra.Command, opts *unmuteOptions, name string) error {
	p := f.Printer

	scope, err := resolveScope(f, cmd, opts.project, opts.job)
	if err != nil {
		return err
	}

	f.Analytics.Track(analytics.GroupTest, analytics.EventTestMuted, map[string]any{
		"action":      analytics.TestActionUnmute,
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

	mutes, err := client.ListMutes(f.Context(), testID, scope)
	if err != nil {
		return err
	}

	if len(mutes.Mute) == 0 {
		return &api.NotFoundError{Resource: "mute", ID: name}
	}

	for _, m := range mutes.Mute {
		if err := client.DeleteMute(f.Context(), m.ID); err != nil {
			return err
		}
	}

	if opts.json {
		return p.PrintJSON(mutes)
	}

	p.Success("Unmuted %s (%d mute(s) removed)", name, len(mutes.Mute))
	return nil
}
