package run

import (
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
