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
		Long:    `List, view, start, and manage TeamCity runs (builds).`,
		Args:    cobra.NoArgs,
		RunE:    cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newRunListCmd(f))
	cmd.AddCommand(newRunViewCmd(f))
	cmd.AddCommand(newRunStartCmd(f))
	cmd.AddCommand(newRunCancelCmd(f))
	cmd.AddCommand(newRunWatchCmd(f))
	cmd.AddCommand(newRunRestartCmd(f))
	cmd.AddCommand(newRunDownloadCmd(f))
	cmd.AddCommand(newRunArtifactsCmd(f))
	cmd.AddCommand(newRunLogCmd(f))
	cmd.AddCommand(newRunPinCmd(f))
	cmd.AddCommand(newRunUnpinCmd(f))
	cmd.AddCommand(newRunTagCmd(f))
	cmd.AddCommand(newRunUntagCmd(f))
	cmd.AddCommand(newRunCommentCmd(f))
	cmd.AddCommand(newRunChangesCmd(f))
	cmd.AddCommand(newRunTestsCmd(f))
	cmd.AddCommand(newRunTreeCmd(f))
	cmd.AddCommand(newRunDiffCmd(f))

	cmdutil.AliasAwareHelp(cmd, "run", "build")
	return cmd
}
