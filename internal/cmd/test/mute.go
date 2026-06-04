package test

import (
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type muteOptions struct {
	project string
	job     string
	reason  string
	until   string
	json    bool
}

func newMuteCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &muteOptions{}

	cmd := &cobra.Command{
		Use:   "mute <test>",
		Short: "Mute a test in a project or job",
		Long: `Mute a test within a project or build configuration.

A scope is required — pass --project or --job. The test name is resolved to its
id; an ambiguous name prints the matching candidates and exits without acting.

--until controls when the mute lifts:
  permanent   stays until removed with 'teamcity test unmute' (default)
  fixed       lifts automatically once the test passes again
  <date>      lifts at the given time (e.g. 2026-01-21 or 2026-01-21T18:00:00)

This change is reversible, so there is no confirmation prompt.

See: https://www.jetbrains.com/help/teamcity/investigating-and-muting-build-failures.html`,
		Example: `  teamcity test mute com.example.FooTest.flaky --project Falcon --reason "flaky, see TC-123"
  teamcity test mute com.example.FooTest.flaky --job Falcon_Build --until fixed
  teamcity test mute com.example.FooTest.flaky --project Falcon --until 2026-01-21`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMute(f, cmd, opts, args[0])
		},
	}

	cmd.Flags().StringVar(&opts.project, "project", "", "Project to mute the test in")
	cmd.Flags().StringVar(&opts.job, "job", "", "Job to mute the test in (takes precedence over --project)")
	cmd.Flags().StringVar(&opts.reason, "reason", "", "Reason recorded with the mute")
	cmd.Flags().StringVar(&opts.until, "until", "permanent", "When the mute lifts: fixed | <date> | permanent")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output the created mute as JSON")

	return cmd
}

func runMute(f *cmdutil.Factory, cmd *cobra.Command, opts *muteOptions, name string) error {
	p := f.Printer

	scope, projectID, err := resolveScope(f, cmd, opts.project, opts.job)
	if err != nil {
		return err
	}

	res, err := parseUntil(opts.until)
	if err != nil {
		return err
	}

	f.Analytics.Track(analytics.GroupTest, analytics.EventTestMuted, map[string]any{
		"action":      analytics.TestActionMute,
		"is_from_job": scope.Job != "",
		"has_reason":  opts.reason != "",
	})

	client, err := f.Client()
	if err != nil {
		return err
	}

	testID, err := resolveTestID(f.Context(), p, client, name, projectID)
	if err != nil {
		return err
	}

	mute, err := client.CreateMute(f.Context(), testID, scope, api.MuteOptions{
		Reason:         opts.reason,
		Resolution:     res.Type,
		ResolutionTime: res.Time,
	})
	if err != nil {
		return err
	}

	if opts.json {
		return p.PrintJSON(mute)
	}

	p.Success("Muted %s (%s)", name, res.describe())
	return nil
}

// muteResolution is the parsed form of --until: a resolution type plus an optional
// absolute time (used only when Type is "atTime").
type muteResolution struct {
	Type string // manually | whenFixed | atTime
	Time string // TeamCity-formatted timestamp when Type == atTime
}

func (r muteResolution) describe() string {
	switch r.Type {
	case "whenFixed":
		return "until fixed"
	case "atTime":
		return "until " + r.Time
	default:
		return "permanent"
	}
}

// untilDateLayouts are the absolute-date formats accepted by --until.
var untilDateLayouts = []string{
	"2006-01-02",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	time.RFC3339,
}

// parseUntil maps the --until flag value to a mute resolution:
//
//	permanent (or empty) → manually (lifts only when unmuted)
//	fixed                → whenFixed (lifts when the test passes)
//	<date>               → atTime (lifts at the parsed time)
func parseUntil(until string) (muteResolution, error) {
	switch strings.ToLower(strings.TrimSpace(until)) {
	case "", "permanent":
		return muteResolution{Type: "manually"}, nil
	case "fixed":
		return muteResolution{Type: "whenFixed"}, nil
	}

	for _, layout := range untilDateLayouts {
		if t, err := time.Parse(layout, until); err == nil {
			return muteResolution{Type: "atTime", Time: api.FormatTeamCityTime(t.UTC())}, nil
		}
	}

	return muteResolution{}, api.Validation(
		fmt.Sprintf("invalid --until value %q", until),
		"use 'fixed', 'permanent', or a date like 2026-01-21",
	)
}
