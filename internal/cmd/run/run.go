package run

import (
	"context"
	"fmt"
	"strconv"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run",
		Aliases: []string{"build"},
		Short:   "Manage runs (builds)",
		Long: `List, view, start, and manage TeamCity runs (builds).

A run (called a build in the TeamCity UI) is a single execution of a
job. Use these commands to trigger runs, watch them live, download
artifacts and logs, inspect test results and VCS changes, and manage
run metadata (tags, comments, pins).

See: https://www.jetbrains.com/help/teamcity/build-results-page.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "LIFECYCLE"},
		&cobra.Group{ID: "artifacts", Title: "ARTIFACTS & LOGS"},
		&cobra.Group{ID: "metadata", Title: "METADATA"},
		&cobra.Group{ID: "analysis", Title: "ANALYSIS"},
	)

	addInGroup := func(groupID string, cmds ...*cobra.Command) {
		for _, c := range cmds {
			c.GroupID = groupID
			cmd.AddCommand(c)
		}
	}

	addInGroup("lifecycle",
		newRunListCmd(f),
		newRunViewCmd(f),
		newRunStartCmd(f),
		newRunCancelCmd(f),
		newRunWatchCmd(f),
		newRunRestartCmd(f),
		newRunDiffCmd(f),
		newRunTreeCmd(f),
	)
	addInGroup("artifacts",
		newRunArtifactsCmd(f),
		newRunDownloadCmd(f),
		newRunLogCmd(f),
	)
	addInGroup("metadata",
		newRunPinCmd(f),
		newRunUnpinCmd(f),
		newRunTagCmd(f),
		newRunUntagCmd(f),
		newRunCommentCmd(f),
	)
	addInGroup("analysis",
		newRunChangesCmd(f),
		newRunTestsCmd(f),
	)

	cmdutil.AliasAwareHelp(cmd, "run", "build")
	return cmd
}

// resolveRunID returns runID when set, else looks up the latest run of jobID (constrained by state).
// The build is also returned so callers can show "#<num>" details. Either runID or jobID must be set;
// otherwise we return a Validation error pointing at the link path.
func resolveRunID(ctx context.Context, client api.ClientInterface, runID, jobID, state string) (string, *api.Build, error) {
	if jobID != "" {
		runs, err := client.GetBuilds(ctx, api.BuildsOptions{
			BuildTypeID: jobID,
			State:       state,
			Limit:       1,
		})
		if err != nil {
			return "", nil, err
		}
		if runs.Count == 0 || len(runs.Builds) == 0 {
			return "", nil, api.Validation(
				fmt.Sprintf("no runs found for job %q", jobID),
				"Try --all, or verify the job ID with 'teamcity job list'",
			)
		}
		b := &runs.Builds[0]
		return strconv.Itoa(b.ID), b, nil
	}
	if runID == "" {
		return "", nil, api.Validation(
			"run ID required",
			"Pass <id>, use --job to get the latest run, or run 'teamcity link' to bind a default job",
		)
	}
	return runID, nil, nil
}
